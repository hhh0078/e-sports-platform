/** 文件作用：独立转写每条连续旁白，以Whisper真实词级DTW时间戳对齐两份配音文案；低覆盖、漏句或乱序时保存诊断并失败，禁止均分时长补造边界。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { spawn } from 'node:child_process'

const root = process.cwd()
const directory = path.resolve(process.env.CONSISTENT_VOICE_ROOT || path.join(root, '.run/consistent-voice'))
const python = process.env.CONSISTENT_ASR_PYTHON || path.join(root, '.run/marketing-tts-env/bin/python')
const model = path.resolve(process.env.CONSISTENT_ASR_MODEL || path.join(root, '.run/marketing-asr-small-model'))
const selfTest = process.argv.includes('--self-test')
const ids = process.argv.find(value => value.startsWith('--ids='))?.slice(6) || process.env.CONSISTENT_VOICE_IDS || ''
const sources = [path.join(root, 'scripts/marketing-voice-script.json'), path.join(root, 'scripts/tutorial-voice-script.json')]
assert.ok(fs.existsSync(python), '缺少本地MLX Python环境')
assert.ok(selfTest || fs.existsSync(path.join(model, 'model.safetensors')), '缺少已缓存Whisper small模型')
fs.mkdirSync(directory, { recursive: true })

// Python仅适配本机MLX推理；匹配过程不把文案传给ASR，也不修改源WAV。
const worker = String.raw`"""文件作用：从整段真实音频独立取得词时间戳，再全局匹配句子；所有时间边界均能追溯到识别词。"""
import hashlib
import json
import math
import os
from pathlib import Path
import re
import subprocess
import sys
import time
import unicodedata

ROOT = Path(os.environ['CONSISTENT_VOICE_ROOT'])
MODEL = Path(os.environ['CONSISTENT_ASR_MODEL'])
MIN_COVERAGE = float(os.environ.get('CONSISTENT_ALIGNMENT_MIN_COVERAGE', '0.75'))
assert 0.5 <= MIN_COVERAGE <= 1, '覆盖率阈值需在0.5至1之间'
METHOD = 'whisper-independent-word-dtw-global-character-edit-alignment-v1'
CANONICAL = {}


def digest(file):
    """分块计算输入摘要，避免把已变动音频或模型的旧识别结果当成当前证据。"""
    result = hashlib.sha256()
    with Path(file).open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            result.update(chunk)
    return result.hexdigest()


def save(file, value):
    """原子保存本地诊断，失败时不遗留上一轮看似成功的对齐文件。"""
    temporary = file.with_suffix(file.suffix + '.tmp')
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')
    temporary.replace(file)


def prepare_canonical(values):
    """一次批量调用macOS内置繁转简；仅建立匹配文字映射，保留原始转写和所有时间。"""
    values=list(dict.fromkeys(values))
    source=ROOT/'canonical-input.json'
    save(source,values)
    javascript='ObjC.import("Foundation"); function run(argv) { var s=$.NSString.stringWithContentsOfFileEncodingError(argv[0], $.NSUTF8StringEncoding, null); var a=JSON.parse(ObjC.unwrap(s)); return JSON.stringify(a.map(function(v) { return ObjC.unwrap($.NSString.stringWithString(v).stringByApplyingTransformReverse("Traditional-Simplified", false)); })); }'
    converted=json.loads(subprocess.run(['/usr/bin/osascript','-l','JavaScript','-e',javascript,str(source)],check=True,capture_output=True,text=True).stdout)
    assert len(converted)==len(values) and all(isinstance(value,str) for value in converted),'系统繁简转换结果异常'
    CANONICAL.update(zip(values,converted))
    save(ROOT/'canonical-texts.json',{'method':'macOS Foundation Traditional-Simplified; NFKC; 12→十二,7→七','pairs':[{'raw':a,'canonical':b} for a,b in zip(values,converted)],'timestamps_modified':False})


def normalized(value):
    """仅规范繁简、全半角与已出现的数字写法；近音误字保留在诊断中。"""
    value=unicodedata.normalize('NFKC',CANONICAL.get(value,value)).lower()
    value=re.sub(r'(?<![0-9])(12|7)(?![0-9])',lambda match:{'12':'十二','7':'七'}[match.group()],value)
    return ''.join(char for char in value if char.isalnum())


def evidence_words(plan,raw,audio_hash):
    """按音频摘要绑定的独立窗口证据替换争议区间，原始ASR只读，绝不改写词级时间。"""
    config_file=ROOT/'asr-evidence-overrides.json'
    words=raw['words']
    applied=[]
    if not config_file.exists():return words,applied
    for item in json.loads(config_file.read_text()):
        if item['id']!=plan['id'] or item['audio_sha256']!=audio_hash:continue
        source=Path(item['evidence_file'])
        assert digest(source)==item['evidence_sha256'],'独立复核文件变化，不能沿用旧窗口处理'
        evidence=json.loads(source.read_text())[item['evidence_index']]
        assert evidence['id']==plan['id'] and evidence['audio_sha256']==audio_hash and evidence['reference_text_supplied'] is False
        start,end=item['replace_start'],item['replace_end']
        selected=[evidence['words'][index] for index in item['word_indices']]
        assert selected and all(0<=word['start']<word['end']<=raw['source_duration_seconds'] for word in selected),'复核词没有正时长'
        assert all(selected[index]['end']<=selected[index+1]['start']+.02 for index in range(len(selected)-1)),'复核词不单调'
        words=[word for word in words if not(start<=word['start']<end)]+selected
        words.sort(key=lambda word:(word['start'],word['end']))
        applied.append(item)
    return words,applied


def resolve_nearphone_review(result,index,reason):
    """只接受有独立复核的完整近音句，保留原字面覆盖率；缺字、零时长或乱序不能豁免。"""
    sentence=result['diagnostics']['sentences'][index]
    assert not result['diagnostics']['global_issues'] and sentence['mapped_character_ratio']==1.0,'真实缺字或全局异常不得作为近音放行'
    assert sentence['issues']==['正确字符覆盖率低于阈值'],'仅可解决完整句的近音字面覆盖差异'
    assert sentence['candidate_start'] is not None and sentence['candidate_end']>sentence['candidate_start']
    sentence['nearphone_review']={'accepted':True,'reason':reason,'literal_coverage_retained':True}
    result['segments'][index].update({'start':round(sentence['candidate_start'],3),'end':round(sentence['candidate_end'],3)})
    result['needs_review']=bool(result['diagnostics']['global_issues'] or any(item['issues'] and not item.get('nearphone_review',{}).get('accepted') for item in result['diagnostics']['sentences']))


def reviewed_sentences(plan,result,audio_hash):
    """复用明确记录且绑定原件摘要的逐句语言复核，不把近音规则扩展到其它句子。"""
    file=ROOT/'asr-sentence-reviews.json'
    applied=[]
    if not file.exists():return applied
    for item in json.loads(file.read_text()):
        if item['id']!=plan['id'] or item['audio_sha256']!=audio_hash:continue
        assert item['text']==plan['segments'][item['index']],'复核原文已变化'
        source=Path(item['evidence_file'])
        assert digest(source)==item['evidence_sha256'],'独立复核证据已变化'
        evidence=json.loads(source.read_text())[item['evidence_index']]
        assert evidence['audio_sha256']==audio_hash and evidence['id']==plan['id'] and evidence['reference_text_supplied'] is False
        resolve_nearphone_review(result,item['index'],item['reason'])
        applied.append(item)
    return applied


def character_alignment(expected, actual):
    """用全局最短编辑路径单调匹配字符，返回真实字符对应关系及未匹配位置。"""
    rows, columns = len(expected), len(actual)
    trace = [bytearray(columns + 1) for _ in range(rows + 1)]
    trace[0][:] = bytes([2]) * (columns + 1)
    previous = list(range(columns + 1))
    for row in range(1, rows + 1):
        trace[row][0] = 1
        current = [row] + [0] * columns
        for column in range(1, columns + 1):
            diagonal = previous[column - 1] + (expected[row - 1] != actual[column - 1])
            deletion, insertion = previous[column] + 1, current[column - 1] + 1
            current[column] = min(diagonal, deletion, insertion)
            trace[row][column] = 0 if diagonal == current[column] else 1 if deletion == current[column] else 2
        previous = current
    row, column = rows, columns
    pairs, deletions, insertions = [], [], []
    while row or column:
        action = trace[row][column]
        if row and column and action == 0:
            pairs.append((row - 1, column - 1, expected[row - 1] == actual[column - 1]))
            row -= 1
            column -= 1
        elif row and (not column or action == 1):
            deletions.append(row - 1)
            row -= 1
        else:
            insertions.append(column - 1)
            column -= 1
    return {'pairs': list(reversed(pairs)), 'deletions': list(reversed(deletions)), 'insertions': list(reversed(insertions)), 'edit_distance': previous[-1]}


def align_sentences(texts, words, duration):
    """句边界只取已匹配词的起止；低覆盖和不单调结果写空边界，不猜测漏掉句子的时间。"""
    expected_parts = [normalized(text) for text in texts]
    assert all(expected_parts), '每句文案必须包含可识别文字'
    expected = ''.join(expected_parts)
    actual_parts = [normalized(word['word']) for word in words]
    actual = ''.join(actual_parts)
    owner = [index for index, part in enumerate(actual_parts) for _ in part]
    result = character_alignment(expected, actual)
    global_issues = []
    if not actual:
        global_issues.append('没有取得有效的真实识别词')
    if len(result['insertions']) > max(3, math.ceil(len(expected) * 0.06)):
        global_issues.append('识别内容存在较多额外文字，需核对重复或跑偏')
    previous_start = -1.0
    for index, word in enumerate(words):
        start, end = float(word['start']), float(word['end'])
        if not math.isfinite(start) or not math.isfinite(end) or start < 0 or end < start or end > duration + 0.05:
            global_issues.append('词时间戳越界：' + str(index))
        if start < previous_start - 0.02:
            global_issues.append('词时间戳不是单调顺序：' + str(index))
        previous_start = start
    segments, diagnostics, offset, previous_end = [], [], 0, -1.0
    for index, (text, part) in enumerate(zip(texts, expected_parts)):
        pairs = [item for item in result['pairs'] if offset <= item[0] < offset + len(part)]
        exact = sum(item[2] for item in pairs)
        coverage = exact / len(part)
        mapped = len(pairs) / len(part)
        issues = []
        candidate_start, candidate_end = None, None
        first_word, last_word, recognized = None, None, ''
        if pairs:
            first_word, last_word = owner[pairs[0][1]], owner[pairs[-1][1]]
            candidate_start = float(words[first_word]['start'])
            candidate_end = min(duration, float(words[last_word]['end']))
            recognized = ''.join(word['word'] for word in words[first_word:last_word + 1]).strip()
            if pairs[0][0] != offset or pairs[-1][0] != offset + len(part) - 1:
                issues.append('句首或句尾没有实际识别字符对应，不猜测边界')
            if candidate_end <= candidate_start:
                issues.append('句子没有正时长的词边界')
            if candidate_start < previous_end - 0.02:
                issues.append('句子与前句边界重叠，需人工核对分句')
        else:
            issues.append('整句没有匹配到真实识别词')
        if coverage < MIN_COVERAGE:
            issues.append('正确字符覆盖率低于阈值')
        if mapped < 0.90:
            issues.append('较多文案字符在识别结果中缺失')
        if pairs:
            zero_words = [number for number in range(first_word, last_word + 1) if actual_parts[number] and float(words[number]['end']) <= float(words[number]['start'])]
            if zero_words:
                issues.append('句内存在零时长识别词，无法承诺完整词边界')
            previous_end = max(previous_end, candidate_end)
        needs_review = bool(issues or global_issues)
        segments.append({'index': index, 'text': text, 'start': None if needs_review else round(candidate_start, 3), 'end': None if needs_review else round(candidate_end, 3), 'coverage': round(coverage, 4)})
        diagnostics.append({'index': index, 'recognized': recognized, 'coverage': round(coverage, 4), 'mapped_character_ratio': round(mapped, 4), 'candidate_start': candidate_start, 'candidate_end': candidate_end, 'first_word_index': first_word, 'last_word_index': last_word, 'issues': issues})
        offset += len(part)
    return {'segments': segments, 'needs_review': bool(global_issues or any(item['issues'] for item in diagnostics)), 'diagnostics': {'global_issues': global_issues, 'sentences': diagnostics, 'expected_normalized': expected, 'recognized_normalized': actual, 'edit_distance': result['edit_distance'], 'unmatched_expected_positions': result['deletions'], 'unmatched_recognized_positions': result['insertions']}}


def self_test():
    """以明确的模拟词时间戳测试纯匹配算法，不生成音频或冒充真实ASR验收。"""
    def fixture(text):
        """构造只用于算法测试的逐字时间戳，真实流程完全使用模型返回的词时间。"""
        return [{'word': char, 'start': index * 0.12, 'end': (index + 1) * 0.12, 'probability': 1.0} for index, char in enumerate(normalized(text))]
    prepare_canonical(['遊戲訂單服務資金','游戏订单服务资金','12元','十二元','7天','七天'])
    assert normalized('遊戲訂單服務資金')==normalized('游戏订单服务资金')
    assert normalized('12元')==normalized('十二元') and normalized('7天')==normalized('七天')
    assert normalized('12元')!=normalized('7元'),'不同金额不能被数字规范化合并'
    first, second = '订单进度和约定，随时查看。', '先聊清需求，再确认服务。'
    homophone = align_sentences([first, second], fixture('订单今度和约定随时查看先聊清需求再确认服务'), 20)
    assert not homophone['needs_review'], homophone
    assert homophone['segments'][0]['start'] == 0 and homophone['segments'][0]['end'] == 1.32
    omitted = align_sentences([first, second], fixture(second), 20)
    assert omitted['needs_review'] and omitted['segments'][0]['start'] is None, omitted
    unordered = align_sentences([first, second], fixture(second + first), 20)
    assert unordered['needs_review'], unordered
    duplicated = align_sentences([first, second], fixture(first + first + second), 20)
    assert duplicated['needs_review'], duplicated
    invalid_words = fixture(first + second)
    invalid_words[-1]['start'] = 0
    invalid = align_sentences([first, second], invalid_words, 20)
    assert invalid['needs_review'] and all(item['start'] is None for item in invalid['segments'])
    try:
        resolve_nearphone_review(omitted,0,'测试不能豁免整句遗漏')
        raise RuntimeError('错误地放行了真实遗漏')
    except AssertionError:pass
    reviewed=align_sentences(['佣金押金分开记'],fixture('拥金押金分开计'),10)
    assert reviewed['needs_review']
    resolve_nearphone_review(reviewed,0,'纯匹配测试：完整同音字，保留字面覆盖率与真实模拟词边界')
    assert not reviewed['needs_review'] and reviewed['segments'][0]['coverage']==.7143
    print('纯匹配测试通过：中文近音误字、整句遗漏、句序颠倒、重复内容及时间戳乱序；没有据此生成任何真实音频对齐文件。', flush=True)


def read_audio(file):
    """按实际WAV采样率解码并等比例重采样16kHz，不改变音频时长或切成逐句推理。"""
    import numpy as np
    from scipy.io import wavfile
    from scipy.signal import resample_poly
    rate, samples = wavfile.read(file)
    assert samples.size and rate > 0, '源WAV没有有效采样'
    original_frames = len(samples)
    if np.issubdtype(samples.dtype, np.integer):
        limits = np.iinfo(samples.dtype)
        samples = (samples.astype(np.float32) - (128 if limits.min == 0 else 0)) / (128 if limits.min == 0 else max(abs(limits.min), limits.max))
    else:
        samples = samples.astype(np.float32)
    if samples.ndim == 2:
        samples = np.mean(samples, axis=1)
    assert samples.ndim == 1 and np.isfinite(samples).all() and float(np.max(np.abs(samples))) > 0.005, '源WAV无有效声音'
    if rate != 16000:
        common = math.gcd(int(rate), 16000)
        samples = resample_poly(samples, 16000 // common, int(rate) // common).astype(np.float32)
    return samples, original_frames / rate


def transcribe(file, audio_hash, model_signature, model_holder):
    """整段音频独立转写一次并保留原始词证据；缓存仅在音频、模型和参数完全相同时复用。"""
    raw_file = file.parent / 'asr-words.json'
    options = {'language': 'zh', 'temperature': 0.0, 'condition_on_previous_text': False, 'return_timestamps': True, 'word_timestamps': True, 'initial_prompt': None, 'hotwords': None}
    cache_key = {'audio_sha256': audio_hash, 'model_signature': model_signature, 'options': options, 'method': METHOD}
    if raw_file.exists() and os.environ.get('CONSISTENT_ALIGNMENT_REFRESH') != '1':
        cached = json.loads(raw_file.read_text())
        if cached.get('cache_key') == cache_key:
            return cached, raw_file
    from mlx_audio.stt import load
    import mlx.core as mx
    samples, duration = read_audio(file)
    if not model_holder:
        model_holder.append(load(str(MODEL), model_type='whisper', strict=True))
    model = model_holder[0]
    assert model.alignment_heads.shape[0] > 0, '模型没有DTW对齐头'
    started = time.monotonic()
    # 明确不提供参考文案、关键词或历史文本提示；原文仅在转写完成后参与字符匹配。
    result = model.generate(samples, **options, verbose=None)
    segments = result.segments or []
    words = [dict(word) for segment in segments for word in segment.get('words', [])]
    assert words, 'Whisper没有返回词级时间戳，禁止用分段或均分替代'
    assert digest(file) == audio_hash, '转写期间源WAV发生变化，请等待TTS完成后重试'
    report = {'cache_key': cache_key, 'audio_file': str(file), 'source_duration_seconds': duration, 'model_path': str(MODEL), 'alignment_heads': model.alignment_heads.tolist(), 'timestamp_resolution_seconds': 0.02, 'independent_asr': True, 'reference_text_supplied': False, 'text': result.text, 'segments': segments, 'words': words, 'elapsed_seconds': round(time.monotonic() - started, 3)}
    save(raw_file, report)
    mx.clear_cache()
    return report, raw_file


def run():
    """串行处理所选七条连续旁白；任何低覆盖结果保留诊断并以非零退出，供合成阶段拒绝使用。"""
    selected = set(filter(None, os.environ.get('CONSISTENT_VOICE_IDS', '').split(',')))
    plans, source_hashes = [], {}
    for source_name in json.loads(os.environ['CONSISTENT_SCRIPT_SOURCES']):
        source = Path(source_name)
        source_hashes[str(source)] = digest(source)
        config = json.loads(source.read_text())
        for plan in config['videos']:
            if not selected or plan['id'] in selected:
                assert plan['id'].replace('-', '').replace('_', '').isalnum(), '非法视频编号'
                assert plan.get('segments') and all(isinstance(text, str) and text.strip() for text in plan['segments']), '无效配音文案'
                plans.append(plan)
    assert plans and len({plan['id'] for plan in plans}) == len(plans), '未找到指定视频或编号重复'
    assert not selected or selected == {plan['id'] for plan in plans}, '存在未知视频编号'
    for plan in plans:
        assert (ROOT / plan['id'] / 'narration.wav').is_file(), '连续旁白尚未生成：' + plan['id']
    model_files = [MODEL / 'config.json', MODEL / 'generation_config.json', MODEL / 'model.safetensors']
    model_signature = hashlib.sha256(json.dumps([(file.name, digest(file)) for file in model_files]).encode()).hexdigest()
    holder, failed, buffered = [], [], []
    for plan in plans:
        file = ROOT / plan['id'] / 'narration.wav'
        audio_hash = digest(file)
        pending = {'video_id': plan['id'], 'method': METHOD, 'source_audio': str(file), 'audio_sha256': audio_hash, 'script_sha256': source_hashes, 'source_duration_seconds': None, 'needs_review': True, 'raw_asr_path': str(file.parent / 'asr-words.json'), 'segments': [{'index': index, 'text': text, 'start': None, 'end': None, 'coverage': 0.0} for index, text in enumerate(plan['segments'])], 'status': 'pending'}
        save(file.parent / 'alignment.json', pending)
        try:
            raw, raw_file = transcribe(file, audio_hash, model_signature, holder)
        except Exception as error:
            pending.update({'status': 'failed', 'error': str(error)})
            save(file.parent / 'alignment.json', pending)
            save(file.parent / 'alignment-diagnostics.json', {'error': str(error), 'boundary': '真实转写没有完成，未生成任何句子时间边界。'})
            raise
        words,overrides=evidence_words(plan,raw,audio_hash)
        buffered.append((plan,file,audio_hash,raw,raw_file,words,overrides))
    prepare_canonical([text for plan,_,_,_,_,_,_ in buffered for text in plan['segments']]+[word['word'] for _,_,_,raw,_,words,_ in buffered for word in raw['words']+words])
    for plan,file,audio_hash,raw,raw_file,words,overrides in buffered:
        result = align_sentences(plan['segments'], words, raw['source_duration_seconds'])
        reviews=reviewed_sentences(plan,result,audio_hash)
        assert digest(file) == audio_hash and all(digest(Path(name)) == value for name, value in source_hashes.items()), '对齐期间输入发生变化'
        report = {'video_id': plan['id'], 'method': METHOD, 'source_audio': str(file), 'audio_sha256': audio_hash, 'script_sha256': source_hashes, 'source_duration_seconds': raw['source_duration_seconds'], 'needs_review': result['needs_review'], 'raw_asr_path': str(raw_file), 'segments': result['segments'], 'coverage_threshold': MIN_COVERAGE, 'canonicalization':'macOS Foundation繁转简、NFKC及12/7数字写法；不改原始证据或时间。','independent_window_evidence':overrides,'reviewed_near_homophones':reviews,'boundary_source': '每句首尾匹配字符所属的真实Whisper词起止；独立窗口替换保留来源，不按字符比例切分词，不插值，不均分时长。', 'timing_precision_note': 'Whisper cross-attention/DTW估计值，原生时间格20毫秒；不是采样级声学真值。'}
        save(file.parent / 'alignment-diagnostics.json', result['diagnostics'])
        save(file.parent / 'alignment.json', report)
        print(json.dumps({'id': plan['id'], 'segments': len(report['segments']), 'minimum_coverage': min(item['coverage'] for item in report['segments']), 'duration_seconds': round(report['source_duration_seconds'], 3), 'needs_review': report['needs_review'], 'alignment': str(file.parent / 'alignment.json')}, ensure_ascii=False), flush=True)
        if report['needs_review']:
            failed.append(plan['id'])
    if failed:
        raise RuntimeError('对齐质量未通过，已保留真实识别与诊断，禁止直接合成：' + ', '.join(failed))


if __name__ == '__main__':
    if '--self-test' in sys.argv:
        self_test()
    else:
        run()
`

const script = path.join(directory, 'align-worker.py')
fs.writeFileSync(script, worker)
const child = spawn(python, [script, ...(selfTest ? ['--self-test'] : [])], {
  stdio: 'inherit',
  env: { ...process.env, CONSISTENT_VOICE_ROOT: directory, CONSISTENT_ASR_MODEL: model, CONSISTENT_VOICE_IDS: ids, CONSISTENT_SCRIPT_SOURCES: JSON.stringify(sources), HF_HUB_OFFLINE: '1', TRANSFORMERS_OFFLINE: '1', HF_DATASETS_OFFLINE: '1', TOKENIZERS_PARALLELISM: 'false' },
})
child.on('error', error => { console.error(error); process.exitCode = 1 })
child.on('exit', code => { process.exitCode = code ?? 1 })
