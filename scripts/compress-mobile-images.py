"""文件作用：压缩内置运营PNG和游戏JPEG供手机展示，保留PNG透明度和原比例，不处理用户上传或业务数据。"""
import io
import json
from pathlib import Path
from PIL import Image

ROOT = Path(__file__).resolve().parents[1]


def compact(path: Path) -> dict:
    """按手机双倍像素生成较小图片，仅在字节减少时替换，PNG仍保留透明背景和半透明边缘。"""
    original = path.read_bytes()
    image = Image.open(io.BytesIO(original))
    before_size = image.size
    output = io.BytesIO()
    if path.suffix == '.png':
        image = image.convert('RGBA')
        width = 768 if 'newcomer' in path.name else 960
        if image.width > width:
            image = image.resize((width, round(image.height * width / image.width)), Image.Resampling.LANCZOS)
        # PNG索引色仍携带透明调色板，保持完整外形和透明背景，不合成底色。
        had_transparency = image.getextrema()[3][0] < 255
        image = image.quantize(colors=256, method=Image.Quantize.FASTOCTREE, dither=Image.Dither.NONE)
        image.save(output, format='PNG', optimize=True, compress_level=9)
        check = Image.open(io.BytesIO(output.getvalue())).convert('RGBA')
        assert not had_transparency or check.getextrema()[3][0] == 0
    else:
        image = image.convert('RGB')
        image.save(output, format='JPEG', quality=66, optimize=True, progressive=True)
    encoded = output.getvalue()
    if len(encoded) < len(original):
        path.write_bytes(encoded)
    else:
        encoded = original
        image = Image.open(io.BytesIO(encoded))
    return {'path': str(path.relative_to(ROOT)), 'before_bytes': len(original), 'after_bytes': len(encoded), 'before_size': before_size, 'after_size': image.size}


def main() -> None:
    """处理受控素材白名单并写出实际节省清单，图标和证据文件不进入压缩范围。"""
    paths = sorted((ROOT / 'backend/internal/assets/operations').glob('*.png'))
    paths += sorted(path for path in (ROOT / 'backend/internal/assets/covers').rglob('*.jpg') if '-icon' not in path.name)
    rows = [compact(path) for path in paths]
    result = {'images': rows, 'before_bytes': sum(row['before_bytes'] for row in rows), 'after_bytes': sum(row['after_bytes'] for row in rows)}
    result['saved_percent'] = round((1 - result['after_bytes'] / result['before_bytes']) * 100, 1)
    (ROOT / 'output/mobile-image-delivery/compression.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({key: value for key, value in result.items() if key != 'images'}))
    print(json.dumps(rows[:2]))


if __name__ == '__main__':
    main()
