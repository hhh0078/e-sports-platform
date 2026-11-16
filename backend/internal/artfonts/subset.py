"""文件作用：从标准输入读取私有字体，按指定码点生成保留版权和完整许可证的 WOFF 子集。"""
import io
import sys
from fontTools import subset
from fontTools.ttLib import TTFont


def main():
    """裁剪受 Go 白名单与长度门禁约束的字体，结果写入标准输出。"""
    font = TTFont(io.BytesIO(sys.stdin.buffer.read()), recalcTimestamp=False)
    options = subset.Options()
    options.name_IDs = ["*"]
    options.name_languages = ["*"]
    options.name_legacy = True
    cutter = subset.Subsetter(options=options)
    cutter.populate(text=sys.argv[1])
    cutter.subset(font)
    # 子集内携带完整授权，下载者无需取得服务端原字体即可查看许可证。
    font["name"].setName(sys.argv[2], 13, 3, 1, 0x409)
    font.flavor = "woff"
    output = io.BytesIO()
    font.save(output)
    sys.stdout.buffer.write(output.getvalue())


if __name__ == "__main__":
    main()
