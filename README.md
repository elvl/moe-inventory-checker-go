# moe-inventory-checker-go
MoE のログファイルからメモリーズボックスの枠拡張のログを収集して html に出力します。

[moe-inventory-checker](https://github.com/elvl/moe-inventory-checker/) の Go 移植版です。
Node 環境がなくても exe でうごくよ！
Go わからんなので移植は Gemini 2.5pro にやらせました。

## Options

* `--path` userdata フォルダのパス。出力もここにされます。 (default: "C:/MOE/Master of Epic/userdata") フォルダが見つからない場合、レジストリから MoE のインストール先を探します。

* `--number` ログ番号。 mlog_yy_mm_dd_n.txt の n (default: 0)

* `--limit` ログファイルをいくつまで遡るか。 0 で制限なし (default: 300)

## 備考

html の生成に成功すると、デフォルトのブラウザで開きます。
レジストリは `HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall` を走査して `Master of Epic` から始まる key を探します。通常のインストール方法を経ていない場合はインストール先を検出できないので、オプションでパスを指定してください。
メモリーズボックスの出力先はメッセージ制御で振り分けできないっぽいので、ログ番号は 0 以外にはなりえないかも。