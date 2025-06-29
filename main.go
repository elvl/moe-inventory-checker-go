package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/pkg/browser"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// コマンドライン引数を格納する構造体
var options struct {
	userdataPath string
	logNumber    int
	limit        int
}

// クエストの定義
type Quest struct {
	Key       string
	ShortName string
	Ex        int
	Type      string
}

var quests = []Quest{
	{Key: "２つ星の依頼 ( チップx2 )", ShortName: "2枚", Ex: 2, Type: "inventory"},
	{Key: "３つ星の依頼 ( ワンダー クロース )", ShortName: "布", Ex: 3, Type: "inventory"},
	{Key: "５つ星の依頼 ( アイリーンズ ベル )", ShortName: "鈴", Ex: 5, Type: "inventory"},
	{Key: "ジャネット 銀行拡張1 ( 1,000 Gold )", ShortName: "1k", Ex: 1, Type: "bank"},
	{Key: "ジャネット 銀行拡張2 ( 2,000 Gold )", ShortName: "2k", Ex: 1, Type: "bank"},
	{Key: "ジャネット 銀行拡張3 ( 4,000 Gold )", ShortName: "4k", Ex: 1, Type: "bank"},
	{Key: "ジャネット 銀行拡張4 ( 8,000 Gold )", ShortName: "8k", Ex: 2, Type: "bank"},
	{Key: "ジャネット 銀行拡張5 ( 16,000 Gold )", ShortName: "16k", Ex: 2, Type: "bank"},
	{Key: "ジャネット 銀行拡張6 ( 32,000 Gold )", ShortName: "32k", Ex: 3, Type: "bank"},
	{Key: "ジャネット 銀行拡張7 ( 64,000 Gold )", ShortName: "64k", Ex: 3, Type: "bank"},
	{Key: "キャット 銀行拡張1 ( ティアーズ ドロップ )", ShortName: "ティア", Ex: 2, Type: "bank"},
	{Key: "キャット 銀行拡張2 ( ビサイド ユア ハート )", ShortName: "ビサ", Ex: 2, Type: "bank"},
	{Key: "２つ星の願い ( 各種生産物 )", ShortName: "IVP", Ex: 2, Type: "bank"},
	{Key: "３つ星の願い ( チップx9 )", ShortName: "9枚", Ex: 3, Type: "bank"},
	{Key: "４つ星の願い ( チップx15 )", ShortName: "15枚", Ex: 4, Type: "bank"},
	{Key: "ガータンの銀行拡張 ( ナジャの爪 )", ShortName: "ナジャ", Ex: 4, Type: "bank"},
	{Key: "イルムの銀行拡張 ( キノコの化石 )", ShortName: "茸", Ex: 3, Type: "bank"},
	{Key: "旅商の安全確保 ( サベージ討伐 )", ShortName: "サベ", Ex: 1, Type: "bank"},
	{Key: "王国復興 極秘任務 ( 各種生産物 )", ShortName: "任務", Ex: 1, Type: "bank"},
	{Key: "悪しき贅沢と正義の盗み ( 隠し金庫 )", ShortName: "金庫", Ex: 1, Type: "bank"},
	{Key: "邪竜討伐 ( 千年竜 )", ShortName: "千年", Ex: 2, Type: "bank"},
	{Key: "エルアン宮殿での銀行拡張 ( 賢王の証 )", ShortName: "賢王", Ex: 2, Type: "bank"},
	{Key: "侵略者の調査依頼", ShortName: "蟻", Ex: 3, Type: "bank"},
	{Key: "未確認生物を追え！", ShortName: "ヒト", Ex: 2, Type: "bank"},
	{Key: "研究素材を手に入れろ！", ShortName: "タコ", Ex: 2, Type: "bank"},
	{Key: "新種の生物を発見せよ！", ShortName: "エビ", Ex: 2, Type: "bank"},
	{Key: "ライオン討伐", ShortName: "獅子", Ex: 2, Type: "bank"},
}

// キャラクターごとのデータを保持する構造体
type Character struct {
	DirEntry  fs.DirEntry
	Server    string
	Name      string
	Inventory Status
	Bank      Status
}

// インベントリや銀行の状態を保持する構造体
type Status struct {
	Quests  map[string]bool
	Updated *time.Time
}

// ログファイル情報を保持する構造体
type LogFile struct {
	fs.FileInfo
	Date time.Time
	Path string
}

// 正規表現
var (
	charaDirRegex  = regexp.MustCompile(`(DIAMOND|PEARL|EMERALD)_([^_]+)_$`)
	logRegex       = regexp.MustCompile(`^[\d/]+ [\d:]+ \[ (○|×) \] (.+?) : \+ (\d+)$`)
	inventoryRegex = regexp.MustCompile(`^所持枠拡張数`)
	bankRegex      = regexp.MustCompile(`^銀行枠拡張数`)
)

// Memories Boxパッチの日付 (2009-05-26)
var memoriesboxPatchDate = time.Date(2009, 5, 26, 0, 0, 0, 0, time.UTC)

func main() {
	// コマンドライン引数の設定
	flag.StringVar(&options.userdataPath, "p", "C:/MOE/Master of Epic/userdata", "userdata フォルダのパス")
	flag.StringVar(&options.userdataPath, "path", "C:/MOE/Master of Epic/userdata", "userdata フォルダのパス")
	flag.IntVar(&options.logNumber, "n", 0, "mlog_yy_mm_dd_n.txt の n")
	flag.IntVar(&options.logNumber, "number", 0, "mlog_yy_mm_dd_n.txt の n")
	flag.IntVar(&options.limit, "l", 300, "ログファイルをいくつまで遡るか。 0 で制限なし")
	flag.IntVar(&options.limit, "limit", 300, "ログファイルをいくつまで遡るか。 0 で制限なし")
	flag.Parse()

	// 実行
	if err := run(); err != nil {
		log.Fatalf("エラーが発生しました: %v", err)
	}
}

func run() error {
  // userdataフォルダのパスを確定
  userdataPath, err := findUserdataPath()
  if err != nil {
    return fmt.Errorf("userdata フォルダが見つからないか使用できないため処理を中断しました。\n-p もしくは --path オプションで正しいパスを指定してください。\n詳細: %w", err)
  }

  fmt.Printf(`
      path: %s
log number: mlog_yy_mm_dd_%d.txt (-n もしくは --number オプションで指定)
     limit: %d (-l もしくは --limit オプションで指定)
`, userdataPath, options.logNumber, options.limit)

  // キャラクターリストの取得
  charaList, err := getCharacterList(userdataPath)
  if err != nil {
    return fmt.Errorf("キャラクターリストの取得に失敗しました: %w", err)
  }

  // 各キャラクターのログを読み込む
  for i := range charaList {
    // readLogfiles に userdataPath を渡すように修正
    if err := readLogfiles(&charaList[i], userdataPath); err != nil {
      fmt.Fprintf(os.Stderr, "\n%s のログ読み込み中にエラー: %v\n", charaList[i].Name, err)
    }
  }

  // HTMLを生成して出力
  if err := generateHTML(charaList, userdataPath); err != nil {
    return fmt.Errorf("HTMLの生成に失敗しました: %w", err)
  }

  return nil
}

// userdataのパスを探す
func findUserdataPath() (string, error) {
	// オプションで指定されたパスを試す
	if _, err := os.Stat(options.userdataPath); err == nil {
		return options.userdataPath, nil
	}

	// レジストリから探す
	fmt.Print("レジストリから MoE のインストール先を検出します。")
	installLocation, err := findInstallLocationFromRegistry()
	if err != nil {
		fmt.Print("失敗。\n")
		return "", err
	}
	fmt.Print("成功。\n")

	userdataPath := filepath.Join(installLocation, "userdata")
	if _, err := os.Stat(userdataPath); err == nil {
		return userdataPath, nil
	}

	return "", fmt.Errorf("レジストリから見つけたパス %s が見つかりません", userdataPath)
}

// レジストリからインストール先を探す
func findInstallLocationFromRegistry() (string, error) {
	hives := []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER}
	for _, hive := range hives {
		appKey, err := registry.OpenKey(hive, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, registry.READ)
		if err != nil {
			continue // キーがなければ次へ
		}
		defer appKey.Close()

		keyNames, err := appKey.ReadSubKeyNames(0)
		if err != nil {
			continue
		}

		for _, keyName := range keyNames {
			if strings.HasPrefix(keyName, "Master of Epic") {
				moeKey, err := registry.OpenKey(appKey, keyName, registry.READ)
				if err != nil {
					continue
				}
				defer moeKey.Close()

				installLocation, _, err := moeKey.GetStringValue("InstallLocation")
				if err == nil && installLocation != "" {
					return installLocation, nil
				}
			}
		}
	}
	return "", errors.New("MoEのインストール情報がレジストリに見つかりませんでした")
}

// キャラクターリストを取得
func getCharacterList(userdataPath string) ([]Character, error) {
	entries, err := os.ReadDir(userdataPath)
	if err != nil {
		return nil, err
	}

	var charaList []Character
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m := charaDirRegex.FindStringSubmatch(entry.Name())
		if m != nil {
			charaList = append(charaList, Character{
				DirEntry:  entry,
				Server:    m[1],
				Name:      strings.ReplaceAll(m[2], ".", ""),
				Inventory: Status{Quests: make(map[string]bool)},
				Bank:      Status{Quests: make(map[string]bool)},
			})
		}
	}

	sort.Slice(charaList, func(i, j int) bool {
		return strings.ToUpper(charaList[i].Name) < strings.ToUpper(charaList[j].Name)
	})

	return charaList, nil
}

// 進捗表示を更新
func updateProgress(chara *Character, current, total int, filename string) {
	server := runewidth.FillRight(chara.Server, 7)
	name := runewidth.FillLeft(chara.Name, 15)
	progress := fmt.Sprintf("%4d/%-4d", current, total)
	fmt.Printf("\r%s %s %s %s", server, name, progress, filename)
}

// ログファイルを読み込む
func readLogfiles(chara *Character, userdataPath string) error {
  charaPath := filepath.Join(userdataPath, chara.DirEntry.Name())
  logFileRegex := regexp.MustCompile(fmt.Sprintf(`^mlog_(\d{2})_(\d{2})_(\d{2})_%d\.txt$`, options.logNumber))

  entries, err := os.ReadDir(charaPath)
  if err != nil {
    return err
  }

	var logFiles []LogFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := logFileRegex.FindStringSubmatch(entry.Name())
		if m != nil {
			// 年, 月, 日
			year := 2000 + toInt(m[1])
			month := toInt(m[2])
			day := toInt(m[3])
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)

			if date.After(memoriesboxPatchDate) || date.Equal(memoriesboxPatchDate) {
				info, _ := entry.Info()
				logFiles = append(logFiles, LogFile{
					FileInfo: info,
					Date:     date,
					Path:     filepath.Join(charaPath, entry.Name()),
				})
			}
		}
	}

	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].Date.After(logFiles[j].Date) // 日付で降順ソート
	})

	for i, logFile := range logFiles {
		if (options.limit > 0 && i >= options.limit) || (chara.Inventory.Updated != nil && chara.Bank.Updated != nil) {
			break
		}
		updateProgress(chara, i+1, len(logFiles), logFile.Name())

		file, err := os.Open(logFile.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nファイルが開けません %s: %v\n", logFile.Path, err)
			continue
		}

		// Shift_JISからUTF-8に変換するリーダーを作成
		sjisReader := transform.NewReader(file, japanese.ShiftJIS.NewDecoder())
		scanner := bufio.NewScanner(sjisReader)
		var mode string

		for scanner.Scan() {
			line := scanner.Text()

			if inventoryRegex.MatchString(line) {
				mode = "inventory"
			} else if bankRegex.MatchString(line) {
				mode = "bank"
			}

			if mode == "" {
				continue
			}

			status := &chara.Inventory
			if mode == "bank" {
				status = &chara.Bank
			}

			// 既に新しいデータがあればスキップ
			if status.Updated != nil && logFile.Date.Before(*status.Updated) {
				continue
			}
			status.Updated = &logFile.Date

			m := logRegex.FindStringSubmatch(line)
			if m != nil {
				questName := m[2]
				isCompleted := (m[1] == "○")
				status.Quests[questName] = isCompleted
			}
		}
		file.Close()
	}
	fmt.Print("\n") // プログレスバーの行をクリア
	return nil
}

// 文字列を整数に変換（エラーは無視）
func toInt(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

// HTMLを生成
func generateHTML(charaList []Character, userdataPath string) error {
	// テンプレートに渡すためのデータ構造を構築
	type TemplateCharaData struct {
		Name    string
		Results []string
		Updated string
	}
	type TemplateServerData struct {
		Name      string
		CharaList []TemplateCharaData
	}
	type TemplateData struct {
		Quests []Quest
		Data   []TemplateServerData
	}

	serverDataMap := make(map[string]*TemplateServerData)
	for _, serverName := range []string{"DIAMOND", "PEARL", "EMERALD"} {
		serverDataMap[serverName] = &TemplateServerData{Name: serverName}
	}

	for _, chara := range charaList {
		server := serverDataMap[chara.Server]
		if server == nil {
			continue
		}
		
		results := make([]string, len(quests))
		for i, q := range quests {
			status := chara.Inventory
			if q.Type == "bank" {
				status = chara.Bank
			}
			if completed, ok := status.Quests[q.Key]; ok && completed {
				results[i] = "◯"
			} else {
				results[i] = ""
			}
		}

		var invUpdated, bankUpdated string
		if chara.Inventory.Updated != nil {
			invUpdated = chara.Inventory.Updated.Format("2006-01-02")
		}
		if chara.Bank.Updated != nil {
			bankUpdated = chara.Bank.Updated.Format("2006-01-02")
		}

		server.CharaList = append(server.CharaList, TemplateCharaData{
			Name:    chara.Name,
			Results: results,
			Updated: fmt.Sprintf("%s\n%s", invUpdated, bankUpdated),
		})
	}
	
	templateData := TemplateData{Quests: quests}
	for _, serverName := range []string{"DIAMOND", "PEARL", "EMERALD"} {
		templateData.Data = append(templateData.Data, *serverDataMap[serverName])
	}
	
	// テンプレートをパース
	tmpl, err := template.New("inventory").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("テンプレートのパースに失敗しました: %w", err)
	}

	// HTMLファイルを作成
	outputPath := filepath.Join(userdataPath, "inventory.html")
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("出力ファイル %s の作成に失敗しました: %w", outputPath, err)
	}
	defer file.Close()

	// テンプレートにデータを適用してファイルに書き込む
	if err := tmpl.Execute(file, templateData); err != nil {
		return fmt.Errorf("テンプレートの実行に失敗しました: %w", err)
	}

	fmt.Printf("HTMLを %s に出力しました。\n", outputPath)

	// ブラウザで開く
	if err := browser.OpenFile(outputPath); err != nil {
		fmt.Printf("ブラウザでファイルを開けませんでした: %v\n", err)
	}

	return nil
}

// HTMLテンプレート（Goのtemplate形式）
const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width,initial-scale=1">
    <title>MoE Inventory Checker</title>
    <style type="text/css">
      body { font-size: 12px; margin-bottom: 5em; }
      th { font-weight: normal; background-color: rgba(217, 217, 221, 1.0); padding: 2px; min-width: 2.5em; white-space: nowrap; position: sticky; top: 0; z-index: 1; }
      th:first-of-type, th:last-of-type { min-width: 7em; width: 7em; }
      th abbr { display: block; }
      .ex { color: rgba(0, 0, 30, .5); }
      tbody tr:nth-child(2n) { background-color: rgba(127, 127, 64, .1); }
      tbody tr:nth-child(2n+1) { background-color: rgba(64, 127, 127, .1); }
      tbody tr td:first-of-type { background-color: rgba(127, 127, 127, .15); }
      tbody tr td:last-of-type { white-space: pre; }
      tbody td { text-align: center; padding: 2px; min-width: 2.5em; height: 2.5em; }
    </style>
</head>
<body>
    <h1>MoE Inventory Checker</h1>
    {{ range .Data }}
        <h2>{{ .Name }}</h2>
        <table>
            <thead>
                <tr>
                    <th>name</th>
                    {{ range $.Quests }}
                        <th><abbr title="{{ .Key }}">{{ .ShortName }}</abbr><span class="ex">+{{ .Ex }}</span></th>
                    {{ end }}
                    <th>updated</th>
                </tr>
            </thead>
            <tbody>
            {{ range .CharaList }}
                <tr>
                    <td>{{ .Name }}</td>
                    {{ range .Results }}
                        <td>{{ . }}</td>
                    {{ end }}
                    <td>{{ .Updated }}</td>
                </tr>
            {{ end }}
            </tbody>
        </table>
    {{ end }}
</body>
</html>
`