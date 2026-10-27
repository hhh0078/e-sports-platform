/** 文件作用：把七条视频各自的完整旁白一次性生成，统一Serena女声与整篇上下文，记录实际指令、输入和采样参数，避免逐句重置造成声线变化；只写本机工作目录。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { spawn } from 'node:child_process'

/** 一条视频的完整原文与原始句子顺序。 */
interface VoiceArticle { id: string; source: string; segments: string[]; text: string; actualGenerationText: string; punctuationNormalized: boolean }

const root = process.cwd(), directory = path.resolve(process.env.CONSISTENT_VOICE_OUTPUT_ROOT || path.join(root, '.run/consistent-voice'))
const python = process.env.MARKETING_TTS_PYTHON || path.join(root, '.run/marketing-tts-env/bin/python')
const model = process.env.MARKETING_TTS_MODEL || path.join(root, '.run/marketing-tts-model')
const speaker = 'Serena'
const plainPunctuation = process.env.CONSISTENT_VOICE_PLAIN_PUNCTUATION === '1'
const finishClearly = process.env.CONSISTENT_VOICE_FINISH_CLEARLY === '1'
assert.ok(!plainPunctuation || process.env.CONSISTENT_VOICE_IDS === '03-game-enthusiast', '标点变体仅限已授权的03整篇重试')
assert.ok(!finishClearly || process.env.CONSISTENT_VOICE_IDS === '02-club-tutorial', '末字收句变体仅限已授权的俱乐部教程整篇重试')
const instruction = '年轻清亮的女声，温暖自然。全篇保持同一音色，轻快利落，停顿简短，语调自然，带轻微笑意。' + (plainPunctuation ? '正常语速平实清晰地讲解，句尾干脆。' : '') + (finishClearly ? '完整讲完所有文字，不省略末字，最后一句自然收尾，稍作停顿。' : '')
const parameters = {
  temperature: Number(process.env.CONSISTENT_VOICE_TEMPERATURE || '0.65'), top_p: 0.90, top_k: 50,
  repetition_penalty: Number(process.env.CONSISTENT_VOICE_REPETITION_PENALTY || '1.05'), max_tokens: 2048,
  seed: Number(process.env.CONSISTENT_VOICE_SEED || '20261026'),
}
assert.ok(parameters.temperature >= 0.1 && parameters.temperature <= 1 && Number.isFinite(parameters.temperature), '整篇温度无效')
assert.ok(parameters.repetition_penalty >= 1 && parameters.repetition_penalty <= 1.5 && Number.isFinite(parameters.repetition_penalty), '整篇重复惩罚无效')
assert.ok(Number.isSafeInteger(parameters.seed) && parameters.seed >= 0, '整篇种子无效')

/**
 * 读取两份既有旁白清单，保留七条视频的原文和句子顺序。
 * @returns 已验证的完整旁白，不继承逐句音色、时长或随机种子设置。
 */
function articles(): VoiceArticle[] {
  const result: VoiceArticle[] = [], ids = new Set<string>()
  for (const file of ['scripts/marketing-voice-script.json', 'scripts/tutorial-voice-script.json']) {
    const value: unknown = JSON.parse(fs.readFileSync(path.join(root, file), 'utf8'))
    assert.ok(typeof value === 'object' && value !== null && 'videos' in value && Array.isArray(value.videos), file + '缺少videos数组')
    for (const entry of value.videos as unknown[]) {
      assert.ok(typeof entry === 'object' && entry !== null && 'id' in entry && typeof entry.id === 'string' && /^[a-z0-9-]+$/.test(entry.id) && 'segments' in entry && Array.isArray(entry.segments), file + '旁白结构无效')
      assert.ok(!ids.has(entry.id), '视频编号重复：' + entry.id); ids.add(entry.id)
      const segments = entry.segments.map((sentence: unknown) => { assert.ok(typeof sentence === 'string' && sentence.trim(), '旁白句子不能为空'); return sentence })
      assert.ok(segments.length > 0 && segments.join('').length < 2000, '旁白长度异常')
      const text = segments.join(''), punctuationNormalized = plainPunctuation && entry.id === '03-game-enthusiast'
      const actualGenerationText = punctuationNormalized ? text.replace(/[！？!?]/g, '。') : text
      assert.equal(actualGenerationText.replace(/[^\p{L}\p{N}]/gu, ''), text.replace(/[^\p{L}\p{N}]/gu, ''), '标点变体不能更改文字或顺序')
      result.push({ id: entry.id, source: file, segments, text, actualGenerationText, punctuationNormalized })
    }
  }
  assert.equal(result.length, 7, '必须保留四条宣传视频和三条教程')
  return result
}

/**
 * 写入单次整篇推理的局部MLX适配器，先支持选定一条试听，再复用同一策略完成其余视频。
 * @returns Python子进程，配音仅保存在.run/consistent-voice目录。
 */
function run(): void {
  assert.ok(fs.existsSync(python), '本地MLX环境不存在')
  assert.ok(fs.existsSync(path.join(model, 'model.safetensors')), '本地配音模型不存在')
  fs.mkdirSync(directory, { recursive: true })
  fs.writeFileSync(path.join(directory, 'input.json'), JSON.stringify({ model: 'mlx-community/Qwen3-TTS-12Hz-1.7B-CustomVoice-4bit', license: 'Apache-2.0', speaker, instruction, parameters, articles: articles() }, null, 2) + '\n')
  const worker = String.raw`"""文件作用：固定Serena声线并记录整篇指令，每条完整旁白只调用一次CustomVoice，不按句重置或做音高处理。"""
from pathlib import Path
import hashlib,json,os,time,wave
import numpy as np
import mlx.core as mx
from mlx_audio.tts.utils import load_model

root=Path(os.environ['CONSISTENT_VOICE_ROOT'])
specification=json.loads((root/'input.json').read_text())
parameters=specification['parameters']
selected=set(filter(None,os.environ.get('CONSISTENT_VOICE_IDS','').split(',')))
known={item['id'] for item in specification['articles']}
assert not selected or selected<=known,'视频编号不在七条既有旁白中'
assert specification['speaker']=='Serena','七条必须固定同一个女性预设'
model=load_model(os.environ['MARKETING_TTS_MODEL'])
assert model.config.tts_model_type=='custom_voice' and model.config.tts_model_size=='1b7','必须使用支持同一情绪指令的CustomVoice1.7B'
seed_scope='one_fixed_seed_per_complete_article'

def save_audio(target,audio,rate,name='narration.wav'):
    """保存整篇原始时序PCM，仅整体安全降峰，不截词、不变速、不变调，并实际读回检查。"""
    data=np.asarray(audio,dtype=np.float32).reshape(-1)
    assert len(data)>rate and np.isfinite(data).all(),'整篇输出无效'
    peak=float(np.max(np.abs(data)))
    assert peak>0.005,'整篇没有有效声音'
    gain=min(1.0,0.90/peak)
    pcm=np.round(data*gain*32767).astype('<i2')
    audio_file=target/name
    temporary=target/(audio_file.stem+'.tmp.wav')
    with wave.open(str(temporary),'wb') as output:
        output.setnchannels(1);output.setsampwidth(2);output.setframerate(rate);output.writeframes(pcm.tobytes())
    with wave.open(str(temporary),'rb') as actual:
        assert actual.getframerate()==rate and actual.getnchannels()==1
        decoded=np.frombuffer(actual.readframes(actual.getnframes()),dtype='<i2')
    assert np.array_equal(decoded,pcm),'PCM读回与整篇输出不一致'
    clipped=int(np.count_nonzero(np.abs(decoded.astype(np.int32))>=32767))
    assert clipped==0,'整篇出现削波'
    temporary.replace(audio_file)
    return {'sample_rate':rate,'samples':len(pcm),'duration_seconds':len(pcm)/rate,'peak':float(np.max(np.abs(pcm.astype(np.float32)/32768))),'rms':float(np.sqrt(np.mean((pcm.astype(np.float32)/32768)**2))),'clipped_samples':clipped,'overall_safety_gain':gain,'audio_sha256':hashlib.sha256(audio_file.read_bytes()).hexdigest()}

completed=[]
failed=[]
for article in specification['articles']:
    if selected and article['id'] not in selected:continue
    target=root/article['id'];target.mkdir(exist_ok=True)
    # 默认原文兼容既有摘要，不因新增诊断字段重生六条已接受音源；变体输入单独参与摘要。
    signature_article=article if article['punctuationNormalized'] else {key:value for key,value in article.items() if key not in ('actualGenerationText','punctuationNormalized')}
    signature=hashlib.sha256(json.dumps([signature_article,specification['speaker'],specification['instruction'],parameters,seed_scope],ensure_ascii=False,sort_keys=True).encode()).hexdigest()
    meta_file=target/'metadata.json';audio_file=target/'narration.wav'
    if meta_file.exists() and audio_file.exists():
        cached=json.loads(meta_file.read_text())
        if cached.get('signature')==signature and cached.get('seed_scope')==seed_scope and cached.get('audio_sha256')==hashlib.sha256(audio_file.read_bytes()).hexdigest():
            completed.append(cached);print('复用整篇',article['id'],round(cached['duration_seconds'],2),'秒',flush=True);continue
    is_advertisement=article['source']=='scripts/marketing-voice-script.json'
    token_limit=(1024 if article['punctuationNormalized'] else 512) if is_advertisement else parameters['max_tokens']
    print('开始整篇',article['id'],'字符',len(article['text']),'上限',token_limit,flush=True)
    started=time.monotonic()
    # 整篇开始前只重置本次明确记录的种子，失败采样可按授权整篇重试；句内绝不重置。
    mx.random.seed(parameters['seed'])
    # 每条视频只有这一次API调用，全部句子共享完整上下文和同一声线；禁止分句循环合成。
    results=list(model.generate_custom_voice(text=article['actualGenerationText'],speaker=specification['speaker'],language='Chinese',instruct=specification['instruction'],temperature=parameters['temperature'],top_p=parameters['top_p'],top_k=parameters['top_k'],repetition_penalty=parameters['repetition_penalty'],max_tokens=token_limit,stream=False,verbose=False))
    assert len(results)==1,'非流式整篇应返回一个完整结果，不能逐句拼接'
    result=results[0]
    duration_seconds=result.samples/result.sample_rate
    if result.token_count>=token_limit or (is_advertisement and duration_seconds>36):
        reason='触达上限，不能把可能截断的长文作为完整结果' if result.token_count>=token_limit else '宣传旁白超过36秒，禁止截字或大倍速压缩'
        diagnostic=save_audio(target,result.audio,int(result.sample_rate),'diagnostic-failed.wav')
        diagnostic.update({'id':article['id'],'reason':reason,'text':article['text'],'actual_generation_text':article['actualGenerationText'],'punctuation_normalized':article['punctuationNormalized'],'voice':specification['speaker'],'instruct':specification['instruction'],'sampling_parameters':parameters,'token_limit':token_limit,'token_count':int(result.token_count),'generation_seconds':time.monotonic()-started,'api_calls_per_video':1,'delivery_allowed':False})
        (target/'diagnostic-failed.json').write_text(json.dumps(diagnostic,ensure_ascii=False,indent=2)+'\n')
        failed.append(diagnostic)
        print('整篇拒绝',article['id'],reason,'完整失败音源已保留，不得交付',flush=True)
        mx.clear_cache()
        continue
    report=save_audio(target,result.audio,int(result.sample_rate))
    report.update({'id':article['id'],'signature':signature,'model':specification['model'],'license':specification['license'],'source':article['source'],'voice':specification['speaker'],'instruct':specification['instruction'],'text':article['text'],'actual_generation_text':article['actualGenerationText'],'punctuation_normalized':article['punctuationNormalized'],'segments':article['segments'],'sampling_parameters':parameters,'token_limit':token_limit,'seed_scope':seed_scope,'api_calls_per_video':1,'non_streaming_result_count':len(results),'token_count':int(result.token_count),'natural_eos_before_limit':True,'generation_seconds':time.monotonic()-started,'model_peak_memory_gb':float(result.peak_memory_usage),'postprocess':'整体必要降峰与16位PCM量化；原始时序保留，未剪词、未变速、未变调。'})
    temp_meta=target/'metadata.tmp.json';temp_meta.write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');temp_meta.replace(meta_file)
    (target/'narration.txt').write_text(article['text']+'\n')
    completed.append(report)
    print('整篇完成',article['id'],round(report['duration_seconds'],2),'秒音频',round(report['generation_seconds'],2),'秒生成','tokens',report['token_count'],flush=True)
    mx.clear_cache()
(root/'generation-report.json').write_text(json.dumps({'completed_ids':[r['id'] for r in completed],'failed_ids':[r['id'] for r in failed],'speaker':specification['speaker'],'instruction':specification['instruction'],'parameters':parameters,'api_calls_per_video':1,'delivery_files_untouched':True,'results':completed,'failures':failed},ensure_ascii=False,indent=2)+'\n')
if failed:
    print('批次未全部通过：'+','.join(item['id'] for item in failed),flush=True)
    raise SystemExit(1)
`
  const script = path.join(directory, 'generate-worker.py')
  fs.writeFileSync(script, worker)
  const child = spawn(python, [script], { stdio: 'inherit', env: { ...process.env, CONSISTENT_VOICE_ROOT: directory, MARKETING_TTS_MODEL: model, HF_HUB_OFFLINE: '1', TOKENIZERS_PARALLELISM: 'false' } })
  child.on('error', error => { console.error(error); process.exitCode = 1 })
  child.on('exit', code => { process.exitCode = code ?? 1 })
}

try { run() } catch (error: unknown) { console.error(error instanceof Error ? error.message : String(error)); process.exitCode = 1 }
