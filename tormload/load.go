package templateload

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
	"text/template"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pkg/errors"
	"github.com/suifengpiao14/glob"
)

func AddFromDir(r *template.Template, patten string) (tplNames []string) {
	allFileList, err := glob.GlobDirectory(patten)
	if err != nil {
		err = errors.WithStack(err)
		panic(err)
	}
	old := getTplNames(r)
	template.Must(r.ParseFiles(allFileList...)) // 追加
	new := getTplNames(r)
	tplNames = getDifferenceTplNames(new, old)
	return tplNames
}

func AddFromFS(r *template.Template, fsys fs.FS, patten string) (out []string) {
	allFileList, err := glob.GlobFS(fsys, patten)
	if err != nil {
		err = errors.WithStack(err)
		panic(err)
	}
	old := getTplNames(r)
	r = template.Must(parseFiles(r, readFileFS(fsys), allFileList...)) // 追加
	new := getTplNames(r)
	out = getDifferenceTplNames(new, old)
	return out
}

func AddFromString(r *template.Template, name string, s string) (out []string) {
	var tmpl *template.Template
	old := getTplNames(r)
	if name == r.Name() {
		tmpl = r
	} else {
		tmpl = r.New(name)
	}
	template.Must(tmpl.Parse(s)) // 追加
	new := getTplNames(r)
	out = getDifferenceTplNames(new, old)
	return out
}

func getTplNames(r *template.Template) (tplNames []string) {
	str := r.DefinedTemplates()
	if str == "" {
		return nil
	}
	const prifix = "; defined templates are: "
	const sep = ","
	str = strings.TrimPrefix(str, prifix)
	arr := strings.Split(str, sep)
	tplNames = make([]string, 0)
	for _, tplName := range arr {
		name := strings.TrimSpace(strings.ReplaceAll(tplName, `"`, ""))
		tplNames = append(tplNames, name)
	}

	return tplNames
}

func getDifferenceTplNames(newTplNames, oldTplNames []string) (detalTplNames []string) {
	oldSet := mapset.NewSet[string]()
	oldSet.Append(oldTplNames...)
	newSet := mapset.NewSet[string]()
	newSet.Append(newTplNames...)
	difference := newSet.Difference(oldSet)
	detalTplNames = difference.ToSlice()
	return detalTplNames
}

// 拷贝template 包helper 方法
func readFileFS(fsys fs.FS) func(string) (string, []byte, error) {
	return func(file string) (name string, b []byte, err error) {
		name = path.Base(file)
		b, err = fs.ReadFile(fsys, file)
		return
	}
}

// parseFiles is the helper for the method and function. If the argument
// template is nil, it is created from the first file.
func parseFiles(t *template.Template, readFile func(string) (string, []byte, error), filenames ...string) (*template.Template, error) {
	if len(filenames) == 0 {
		// Not really a problem, but be consistent.
		return nil, fmt.Errorf("template: no files named in call to ParseFiles")
	}
	for _, filename := range filenames {
		name, b, err := readFile(filename)
		if err != nil {
			return nil, err
		}
		s := string(b)
		// First template becomes return value if not already defined,
		// and we use that one for subsequent New calls to associate
		// all the templates together. Also, if this file has the same name
		// as t, this file becomes the contents of t, so
		//  t, err := New(name).Funcs(xxx).ParseFiles(name)
		// works. Otherwise we create a new template associated with t.
		var tmpl *template.Template
		if t == nil {
			t = template.New(name)
		}
		if name == t.Name() {
			tmpl = t
		} else {
			tmpl = t.New(name)
		}
		_, err = tmpl.Parse(s)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}
