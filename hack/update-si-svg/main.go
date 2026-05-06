package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type clArgs struct {
	pathToNodeModule string
	pathToOutputJS   string
	pathToMarkdown   string
}

type iconMetadata struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

type nodeVersion struct {
	Version string `json:"version"`
}

type svg struct {
	XMLName xml.Name
	Height  string `xml:"height,attr"`
	Width   string `xml:"width,attr"`
	Role    string `xml:"role,attr"`
	Viewbox string `xml:"viewBox,attr"`
	Doc     string `xml:",innerxml"`
}

const (
	noIconSVG    = `<svg xmlns="http://www.w3.org/2000/svg" id="mdi-help-circle-outline" viewBox="0 0 24 24"><path d="M11,18H13V16H11V18M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2M12,20C7.59,20 4,16.41 4,12C4,7.59 7.59,4 12,4C16.41,4 20,7.59 20,12C20,16.41 16.41,20 12,20M12,6A4,4 0 0,0 8,10H10A2,2 0 0,1 12,8A2,2 0 0,1 14,10C14,12 11,11.75 11,15H13C13,12.75 16,12.5 16,10A4,4 0 0,0 12,6Z" /></svg>`
	scriptHeader = `export const svg = (function () {var icons = {`
	scriptFooter = `"no-icon":""}; var svg = function(id){return icons[id] ?? '` + noIconSVG + `';}; return svg;})();`
	mdHeader     = `# Simple Icons Support

The following icons are available from [Simple Icons](https://simpleicons.org):

|**Title**|**Slug**|**Preview**|
|---|---|---|`
)

func main() {
	var args clArgs

	flag.StringVar(&args.pathToNodeModule, "path-to-simple-icons-module", "node_modules/simple-icons", "path to the simple-icons node module. If relative, it is relative to the current working directory")
	flag.StringVar(&args.pathToOutputJS, "path-to-output-js", "web/public/si-svg.min.js", "the path to the file to be generated. If relative, it is relative to the current working directory")
	flag.StringVar(&args.pathToMarkdown, "path-to-markdown", "simple-icons.md", "if set, a markdown document containing the title and slug will be generated. if '-', output to stdout")

	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if !filepath.IsAbs(args.pathToNodeModule) {
		args.pathToNodeModule = filepath.Join(wd, args.pathToNodeModule)
	}

	if !filepath.IsAbs(args.pathToOutputJS) {
		args.pathToOutputJS = filepath.Join(wd, args.pathToOutputJS)
	}

	if args.pathToMarkdown != "" && args.pathToMarkdown != "-" && !filepath.IsAbs(args.pathToMarkdown) {
		args.pathToMarkdown = filepath.Join(wd, args.pathToMarkdown)
	}

	pkgJSON, err := os.ReadFile(filepath.Join(args.pathToNodeModule, "package.json"))
	if err != nil {
		panic(err)
	}

	var version nodeVersion
	if err = json.Unmarshal(pkgJSON, &version); err != nil {
		panic(err)
	}

	jsWriter, err := os.Create(args.pathToOutputJS)
	if err != nil {
		panic(err)
	}
	defer jsWriter.Close()

	var mdWriter io.Writer
	switch args.pathToMarkdown {
	case "":
		mdWriter = io.Discard
	case "-":
		mdWriter = os.Stdout
	default:
		mdWriter, err = os.Create(args.pathToMarkdown)
		if err != nil {
			panic(err)
		}
		defer mdWriter.(*os.File).Close()
	}

	fmt.Fprint(jsWriter, scriptHeader)
	fmt.Fprintln(mdWriter, mdHeader)

	var index []iconMetadata
	indexFile, err := os.Open(filepath.Join(args.pathToNodeModule, "data/simple-icons.json"))
	if err != nil {
		panic(err)
	}
	defer indexFile.Close()

	if err = json.NewDecoder(indexFile).Decode(&index); err != nil {
		panic(err)
	}

	for _, icon := range index {
		svg, err := loadSVG(args.pathToNodeModule, icon.Slug)
		if err != nil {
			panic(err)
		}

		fmt.Fprintf(jsWriter, `"si/%s":%q,`, icon.Slug, svg)
		fmt.Fprintf(mdWriter, "|%[1]s|`si/%[2]s`|[Preview](https://raw.githubusercontent.com/simple-icons/simple-icons/refs/tags/%[3]s/icons/%[2]s.svg)|", icon.Title, icon.Slug, version.Version)
		fmt.Fprintln(mdWriter)
	}

	fmt.Fprintln(jsWriter, scriptFooter)
}

func loadSVG(rootDir string, slug string) (string, error) {
	fullPath := filepath.Join(rootDir, "icons", slug+".svg")
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	var s svg
	if err = xml.Unmarshal(data, &s); err != nil {
		return "", err
	}

	s.Height = "24px"
	s.Width = "24px"

	b := new(strings.Builder)
	err = xml.NewEncoder(b).Encode(s)

	return b.String(), err
}
