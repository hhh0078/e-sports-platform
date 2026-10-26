/** 文件作用：用本地Qwen3-TTS情绪模型生成四类受众逐镜头配音；TypeScript负责工具编排，Python仅调用MLX模型，不使用系统声音或真人克隆。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {spawn} from 'node:child_process'
const root=process.cwd(),directory=path.join(root,'.run/marketing-voice')
const python=process.env.MARKETING_TTS_PYTHON||path.join(root,'.run/marketing-tts-env/bin/python')
const model=process.env.MARKETING_TTS_MODEL||path.join(root,'.run/marketing-tts-model')
assert.ok(fs.existsSync(python),'请先准备独立MLX配音环境')
assert.ok(fs.existsSync(path.join(model,'model.safetensors')),'情绪模型尚未下载完成')
fs.mkdirSync(directory,{recursive:true})
fs.copyFileSync(path.join(root,'scripts/marketing-voice-script.json'),path.join(directory,'script.json'))
// Python是MLX官方推理接口所需的局部适配，不进入前端业务或产品运行环境。
const worker=String.raw`"""文件作用：调用本地MLX情绪语音模型生成可复用的逐镜头PCM配音，并记录文本、音色和音量校验信息。"""
import os,json,hashlib,time,wave
from pathlib import Path
import numpy as np
import mlx.core as mx
from mlx_audio.tts.utils import load_model

root=Path(os.environ['MARKETING_VOICE_ROOT'])
config=json.loads((root/'script.json').read_text())
selected=set(filter(None,os.environ.get('MARKETING_VOICE_IDS','').split(',')))
indices=set(int(v) for v in os.environ.get('MARKETING_VOICE_SEGMENTS','').split(',') if v)
attempt=int(os.environ.get('MARKETING_VOICE_ATTEMPT','0'))
model=load_model(os.environ['MARKETING_TTS_MODEL'])
assert model.config.tts_model_size!='0b6','必须使用实际支持情绪指令的1.7B模型'

def write_audio(file,audio,rate):
    """保存非静音的单声道PCM，保留自然音调并验证有效声音和安全峰值。"""
    data=np.asarray(audio,dtype=np.float32).reshape(-1)
    assert data.size>rate/4 and np.isfinite(data).all(),'语音输出无效'
    peak=float(np.max(np.abs(data)))
    assert peak>0.005,'生成结果没有有效声音'
    data=data*min(1.0,0.92/peak)
    pcm=np.round(np.clip(data,-1,1)*32767).astype('<i2')
    with wave.open(str(file),'wb') as wav:
        wav.setnchannels(1);wav.setsampwidth(2);wav.setframerate(rate);wav.writeframes(pcm.tobytes())
    return {'sample_rate':rate,'duration_seconds':len(data)/rate,'peak':float(np.max(np.abs(data))),'rms':float(np.sqrt(np.mean(data*data)))}

for plan_index,plan in enumerate(config['videos']):
    if selected and plan['id'] not in selected:continue
    target=root/plan['id'];target.mkdir(exist_ok=True)
    for index,text in enumerate(plan['segments']):
        if indices and index not in indices:continue
        timing='开头钩子，短促抓耳，请在两秒半左右自然说完。' if index==0 else '这一句请在四秒内自然利落说完，短停顿，重点鲜明。'
        instruction=plan['instruct']+timing
        signature=hashlib.sha256(json.dumps([text,plan['voice'],instruction,attempt],ensure_ascii=False).encode()).hexdigest()
        file=target/(str(index)+'.wav');meta=target/(str(index)+'.json')
        if file.exists() and meta.exists() and json.loads(meta.read_text()).get('signature')==signature:
            print('复用配音',plan['id'],index,flush=True);continue
        mx.random.seed(700+plan_index*100+index+attempt*1000)
        started=time.time()
        chunks=list(model.generate_custom_voice(text=text,speaker=plan['voice'],language='Chinese',instruct=instruction,temperature=0.75,top_p=0.9,repetition_penalty=1.08,max_tokens=256,verbose=False))
        assert chunks,'模型没有返回配音'
        rate=int(chunks[0].sample_rate)
        assert all(int(chunk.sample_rate)==rate for chunk in chunks)
        audio=np.concatenate([np.asarray(chunk.audio,dtype=np.float32).reshape(-1) for chunk in chunks])
        report=write_audio(file,audio,rate)
        report.update({'signature':signature,'text':text,'voice':plan['voice'],'instruct':instruction,'model':config['model'],'license':config['license'],'generation_seconds':time.time()-started})
        meta.write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
        print('配音完成',plan['id'],index,round(report['duration_seconds'],3),'秒',round(report['generation_seconds'],1),'秒生成',flush=True)
        mx.clear_cache()
`
const script=path.join(directory,'generate-worker.py');fs.writeFileSync(script,worker)
const child=spawn(python,[script],{stdio:'inherit',env:{...process.env,MARKETING_VOICE_ROOT:directory,MARKETING_TTS_MODEL:model,HF_HUB_OFFLINE:'1',TOKENIZERS_PARALLELISM:'false'}})
child.on('error',error=>{console.error(error);process.exitCode=1})
child.on('exit',code=>{process.exitCode=code??1})
