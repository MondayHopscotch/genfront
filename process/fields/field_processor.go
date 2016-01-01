package fields

import (
	"go/token"
	"go/parser"
	"go/ast"
	"fmt"
	cmd "github.com/codegangsta/cli"
	"path/filepath"
	"html/template"
	"log"
	"os"
	"strings"
	"github.com/lcaballero/genfront/process"
	"github.com/lcaballero/genfront/cli"
)

type GenState int
const (
	InitialFieldsGen GenState = 1
	HasComment GenState = 2
)

type FieldsProcessor struct {
	*cli.CliConf
}

func NewFieldProcessor(c *cmd.Context) {
	fp := &FieldsProcessor{
		CliConf: cli.NewCliConf(c),
	}

	process.ShowEnvironment()
	fp.Load()
}

func (fp *FieldsProcessor) Validate() bool {
	return fp.CliConf.HasOutputFile()
}

func (fp *FieldsProcessor) Load() {
	env := process.BuildEnv()
	cwd := env.String("CWD")
	gofile := env.String("GOFILE")
	filename := filepath.Join(cwd, gofile)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	line := fp.Line()
	fmt.Printf("GOLINE: %d\n", line)

	state := InitialFieldsGen
	structName := ""

	ast.Inspect(f, func(n ast.Node) bool {

		switch x := n.(type) {
		case *ast.TypeSpec:
		case *ast.Comment:
			file := fset.File(x.Slash)
			pos := file.Position(x.Slash)
			if pos.Line == line {
				state = HasComment
			}
		case *ast.Ident:
			structName = x.Name
		case *ast.StructType:
			if state == HasComment {
				fmt.Println("structName", structName)
				fp.State(filename, structName, x)
				state = InitialFieldsGen
			}
		}
		return true
	})
}

func deriveOutfile(gen string) string {
	ext := filepath.Ext(gen)
	base := filepath.Base(gen)
	noext := base[0:len(base) - len(ext)]
	f := fmt.Sprintf("%s_tomap.go", noext)
	return f
}

func (p *FieldsProcessor) outfile(gen string) string {
	cli := p.OutputFile()

	if cli == "" {
		return deriveOutfile(gen)
	} else {
		return cli
	}
}

func (p *FieldsProcessor) Render() (*template.Template, error) {
	tpl,err := process.Asset("struct_sql_tomap.fm")
	if err != nil {
		return nil, err
	}
	fm := strings.TrimLeft(string(tpl), " \n\r\t")
	return template.New("").Funcs(process.BuildFuncMap()).Parse(fm)
}

func (fp *FieldsProcessor) State(filename, structName string, stc *ast.StructType) {
	names := []string{}
	for _,f := range stc.Fields.List {
		for _, name := range f.Names {
			names = append(names, name.Name)
		}
	}

	tpl, err := fp.Render()
	if err != nil {
		log.Fatal(err)
	}
	env := process.BuildEnv()
	env = process.BuildData(env)
	env["names"] = names
	env["structName"] = structName

	file, err := os.Create(fp.outfile(filename))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	tpl.Execute(file, env)
}