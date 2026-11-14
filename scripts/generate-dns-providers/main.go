// Generate the console catalogue from the exact lego module used by HomeWaf.
// Run from the repository root: go run ./scripts/generate-dns-providers [-check]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type field struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Advanced    bool   `json:"advanced"`
}
type provider struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases"`
	Documentation string   `json:"documentation"`
	Fields        []field  `json:"fields"`
	Instructions  string   `json:"instructions"`
	Example       string   `json:"example"`
	Unavailable   string   `json:"unavailable,omitempty"`
}

func main() {
	check := flag.Bool("check", false, "fail if the generated catalogue is stale")
	flag.Parse()
	raw, err := exec.Command("go", "list", "-m", "-json", "github.com/go-acme/lego/v4").Output()
	must(err)
	var mod struct{ Dir, Version string }
	must(json.Unmarshal(raw, &mod))
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(mod.Dir, "providers/dns/zz_gen_dns_providers.go"), nil, 0)
	must(err)
	var providers []provider
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CaseClause)
		if !ok || len(c.List) == 0 {
			return true
		}
		var aliases []string
		for _, value := range c.List {
			name, err := strconv.Unquote(value.(*ast.BasicLit).Value)
			must(err)
			aliases = append(aliases, name)
		}
		call := c.Body[0].(*ast.ReturnStmt).Results[0].(*ast.CallExpr)
		pkg := call.Fun.(*ast.SelectorExpr).X.(*ast.Ident).Name
		path := filepath.Join(mod.Dir, "providers/dns", pkg, pkg+".toml")
		data, err := os.ReadFile(path)
		must(err)
		var doc struct {
			Name, Code, Additional, Example string
			Configuration                   struct{ Credentials, Additional map[string]string }
		}
		must(toml.Unmarshal(data, &doc))
		p := provider{Code: doc.Code, Name: doc.Name, Aliases: aliases, Fields: []field{},
			Documentation: "https://go-acme.github.io/lego/dns/" + pkg + "/",
			Instructions:  strings.TrimSpace(doc.Additional), Example: strings.TrimSpace(doc.Example)}
		for i, fields := range []map[string]string{doc.Configuration.Credentials, doc.Configuration.Additional} {
			var keys []string
			for key := range fields {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				p.Fields = append(p.Fields, field{key, fields[key], i == 1})
			}
		}
		if pkg == "exec" {
			p.Fields = []field{{"EXEC_PATH", "服务端外部 DNS 更新程序的绝对路径（需自行安装；以 HomeWaf 服务账户运行）", false}, {"EXEC_MODE", "可选 RAW 模式；默认传递操作、TXT 记录名和记录值", true}, {"EXEC_POLLING_INTERVAL", "DNS 检查间隔（秒）", true}, {"EXEC_PROPAGATION_TIMEOUT", "DNS 传播超时（秒）", true}, {"EXEC_SEQUENCE_INTERVAL", "顺序验证间隔（秒）", true}}
		}
		if pkg == "manual" {
			p.Unavailable = "lego 的 manual 需要终端交互，不能用于后台自动申请和续期。请选择域名的 DNS 服务商，或配置 acme-dns 委托。"
		}
		providers = append(providers, p)
		return false
	})
	sort.Slice(providers, func(i, j int) bool { return providers[i].Code < providers[j].Code })
	output, err := json.MarshalIndent(struct {
		Version   string     `json:"version"`
		Providers []provider `json:"providers"`
	}{mod.Version, providers}, "", "  ")
	must(err)
	output = append(output, '\n')
	path := "internal/dnsproviders/catalogue.json"
	if *check {
		old, err := os.ReadFile(path)
		must(err)
		if !bytes.Equal(old, output) {
			panic("DNS catalogue is stale; run go run ./scripts/generate-dns-providers")
		}
	} else {
		must(os.MkdirAll(filepath.Dir(path), 0755))
		must(os.WriteFile(path, output, 0644))
		license, err := os.ReadFile(filepath.Join(mod.Dir, "LICENSE"))
		must(err)
		must(os.WriteFile("internal/dnsproviders/LEGO-LICENSE", license, 0644))
	}
	fmt.Printf("lego %s: %d providers; catalogue verified\n", mod.Version, len(providers))
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
