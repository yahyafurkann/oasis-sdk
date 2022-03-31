// extract-runtime-txs extracts runtime transactions from Rust, Go, and TypeScript sources
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	CfgMarkdown               = "markdown"
	CfgMarkdownTplFile        = "markdown.template.file"
	CfgMarkdownTplPlaceholder = "markdown.template.placeholder"
	CfgCodebasePath           = "codebase.path"
	CfgCodebaseURL            = "codebase.url"
)

var (
	scriptName = filepath.Base(os.Args[0])

	rootCmd = &cobra.Command{
		Use:     scriptName,
		Short:   "Extracts Runtime transactions from formatted Rust, Go and TypeScript code.",
		Long:    `TODO`,
		Example: "./extract-runtime-txs --codebase.path ../.. --markdown",
		Run:     doExtractRuntimeTxs,
	}
)

// refAnchor returns the reference
func refAnchor(l Lang, fullName string, t RefType) string {
	refTypeStr := ""
	if t != Base {
		refTypeStr = fmt.Sprintf("-%s", t)
	}

	return fmt.Sprintf("%s-%s%s", l, fullName, refTypeStr)
}

// markdownRef generates [ Go | Rust | TypeScript ] for the provided snippet.
func markdownRef(fullName string, snippets map[Lang]Snippet, t RefType) string {
	langMarkdown := []string{}
	for lang, _ := range snippets {
		ref := fmt.Sprintf("[%s][%s]", lang.ToString(), refAnchor(lang, fullName, t))
		langMarkdown = append(langMarkdown, ref)
	}

	return fmt.Sprintf("[%s]", strings.Join(langMarkdown, " | "))
}

func markdownParams(params []Parameter) string {
	paramsStr := "\n"
	for _, p := range params {
		paramsStr += fmt.Sprintf("- `%s: %s`\n", p.Name, p.Type)
		if p.Description != "" {
			paramsStr += fmt.Sprintf("\n  %s\n\n", p.Description)
		}
	}
	return paramsStr
}

func snippetPath(s Snippet) string {
	baseDir := viper.GetString(CfgCodebasePath)
	if viper.IsSet(CfgMarkdownTplFile) && !viper.IsSet(CfgCodebaseURL) {
		baseDir = filepath.Dir(viper.GetString(CfgMarkdownTplFile))
	}
	fileURL, _ := filepath.Rel(baseDir, s.Path)
	if viper.IsSet(CfgCodebaseURL) {
		fileURL = viper.GetString(CfgCodebaseURL) + fileURL
	}
	linesStr := ""
	if s.LineFrom != 0 {
		linesStr = fmt.Sprintf("#L%d", s.LineFrom)
		if s.LineTo != s.LineFrom {
			linesStr += fmt.Sprintf("-L%d", s.LineTo)
		}
	}
	return fmt.Sprintf("%s%s", fileURL, linesStr)
}

func markdownList(txs []Tx) string {
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].FullName() < txs[j].FullName()
	})

	tStr := ""
	lastModule := ""
	for _, t := range txs {
		if t.Module != lastModule {
			tStr += fmt.Sprintf("## %s\n\n", t.Module)
			lastModule = t.Module
		}
		tStr += fmt.Sprintf("### %s\n", t.FullName())
		tStr += fmt.Sprintf("(%s) %s\n\n", t.Type, markdownRef(t.FullName(), t.Ref, Base))
		tStr += fmt.Sprintf("#### Parameters %s\n%s\n", markdownRef(t.FullName(), t.ParametersRef, Params), markdownParams(t.Parameters))

		if t.Result != nil {
			tStr += fmt.Sprintf("#### Result %s\n%s\n", markdownRef(t.FullName(), t.ResultRef, Result), markdownParams(t.Result))
		}

		for l, s := range t.Ref {
			tStr += fmt.Sprintf("[%s]: %s\n", refAnchor(l, t.FullName(), Base), snippetPath(s))
		}
		for l, s := range t.ParametersRef {
			tStr += fmt.Sprintf("[%s]: %s\n", refAnchor(l, t.FullName(), Params), snippetPath(s))
		}
		for l, s := range t.ResultRef {
			tStr += fmt.Sprintf("[%s]: %s\n", refAnchor(l, t.FullName(), Result), snippetPath(s))
		}

		tStr += "\n"
	}

	return tStr
}

func printMarkdown(transactions []Tx) {
	markdown := markdownList(transactions)

	if !viper.IsSet(CfgMarkdownTplFile) {
		// Print Markdown only.
		fmt.Print(markdown)
		return
	}

	md, err := ioutil.ReadFile(viper.GetString(CfgMarkdownTplFile))
	if err != nil {
		panic(err)
	}

	mdStr := strings.Replace(string(md), viper.GetString(CfgMarkdownTplPlaceholder)+"\n", markdown, 1)
	fmt.Print(mdStr)
}

func printJSON(m []Tx) {
	data, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", data)
}

var transactions = []Tx{}

func doExtractRuntimeTxs(cmd *cobra.Command, args []string) {
	searchDir := viper.GetString(CfgCodebasePath) + "/runtime-sdk"
	err := filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
		}
		if f.IsDir() {
			return nil
		}
		if !strings.HasSuffix(f.Name(), ".rs") {
			return nil
		}
		rustParser := RustParser{filename: path}
		txs, err := rustParser.FindTransactions()
		if err != nil {
			return err
		}

		transactions = append(transactions, txs...)

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	if viper.GetBool(CfgMarkdown) {
		printMarkdown(transactions)
	} else {
		printJSON(transactions)
	}
}

// extractValue returns string value of the identifier or literal.
func extractValue(n ast.Expr) string {
	lit, ok := n.(*ast.BasicLit)
	if ok {
		// Strip quotes.
		return lit.Value[1 : len(lit.Value)-1]
	}

	ident, ok := n.(*ast.Ident)
	if !ok || ident.Obj == nil {
		return ""
	}
	decl, ok := ident.Obj.Decl.(*ast.ValueSpec)
	if !ok || len(decl.Values) != 1 {
		return ""
	}
	val, ok := decl.Values[0].(*ast.BasicLit)
	if !ok {
		return ""
	}
	// Strip quotes.
	return val.Value[1 : len(val.Value)-1]
}

func main() {
	rootCmd.Flags().Bool(CfgMarkdown, false, "print metrics in markdown format")
	rootCmd.Flags().String(CfgCodebasePath, "", "path to Go codebase")
	rootCmd.Flags().String(CfgCodebaseURL, "", "show URL to Go files with this base instead of relative path (optional) (e.g. https://github.com/oasisprotocol/oasis-sdk/tree/master/)")
	rootCmd.Flags().String(CfgMarkdownTplFile, "", "path to Markdown template file")
	rootCmd.Flags().String(CfgMarkdownTplPlaceholder, "<!--- OASIS_RUNTIME_TRANSACTIONS -->", "placeholder for Markdown table in the template")
	_ = cobra.MarkFlagRequired(rootCmd.Flags(), CfgCodebasePath)
	_ = viper.BindPFlags(rootCmd.Flags())

	_ = rootCmd.Execute()
}
