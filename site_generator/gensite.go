package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

// locations of resources used to build site
const (
	TemplateDir  = "./builders/"
	HTMLDir      = "./html/"
	ChangelogDir = "./changelogs/"
)

type walkstruct map[string]string

func unwrapExit[T any](v T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return v
}

func (H walkstruct) walkerfunc(fpath string, info fs.FileInfo, err error) error {
	if err != nil {
		return err
	}

	fname := info.Name()
	if strings.HasSuffix(fname, ".b.html") && !info.IsDir() {
		cont, err := os.ReadFile(fpath)

		if err != nil {
			return err
		}

		H[fpath] = string(cont)

	}

	return nil
}

func transform(filePath, fileData, temp string, rch *sync.WaitGroup) {
	dat := strings.Split(fileData, "\n")

	if len(dat) <= 3 {
		log.Fatal(filePath, ": File has incomplete BHTML headers")
	}

	if dat[0] == "0" {

		// Parse bhtml headers and content
		title := dat[1]
		content := strings.Join(dat[2:], "\n\t\t\t")

		// Add title and content
		replaces := []string{
			"<TEMPLATE>[HTML-TITLE]</TEMPLATE>", title,
			"<TEMPLATE>[HTML-CONTENT]</TEMPLATE>", content,
		}

		nextemp := strings.NewReplacer(replaces...).Replace(temp)

		newfile := unwrapExit(os.Create(filePath[:len(filePath)-6] + "html"))
		defer newfile.Close()
		unwrapExit(newfile.Write([]byte(nextemp)))
	}
	// Tell the main thread we are done
	rch.Done()
}

func buildTemplates() {

	templates := make(map[string]string)
	tfiles := unwrapExit(os.ReadDir(TemplateDir + "templatedata"))

	for _, f := range tfiles {
		fname := f.Name()
		if strings.HasSuffix(fname, ".t.html") {
			cont, readerr := os.ReadFile(TemplateDir + "templatedata/" + fname)
			if readerr != nil {
				fmt.Println("Error reading file:", fname)
			} else {
				templates[fname[:len(fname)-7]] = string(cont)
			}
		}
	}

	ConvFiles := walkstruct{}
	walkerr := filepath.Walk(HTMLDir, ConvFiles.walkerfunc)

	if walkerr != nil {
		log.Fatal(walkerr)
	}

	templatefile := unwrapExit(os.ReadFile(TemplateDir + "TEMPLATE.t.html"))

	rawstart := string(templatefile)

	// Add templatedata files
	replacements := make([]string, 0, len(templates)*2)
	for k, v := range templates {
		ev := strings.Join(strings.Split(v, "\n"), "\n\t\t\t")
		replacements = append(replacements, "<TEMPLATE>["+k+"]</TEMPLATE>", ev)
	}

	rawstart = strings.NewReplacer(replacements...).Replace(rawstart)

	done := new(sync.WaitGroup)
	done.Add(len(ConvFiles))

	for path, data := range ConvFiles {
		go transform(path, data, rawstart, done)
	}

	// wait for threads to finish
	done.Wait()

}

// converts \`\` codequotes to <code></code>
func codeQuoteHTML(s string) string {

	num_quotes := strings.Count(s, "`")

	// skip allocation if no quotes
	if num_quotes == 0 {
		return s
	}

	inQuote := false

	builder := strings.Builder{}
	// account for length of quotes div 2, for every quote group allocate one code html (-2 for `` being removed)
	builder.Grow(len(s) + (num_quotes / 2 * (len("<code></code>") - 2)))

	for _, ch := range s {
		if ch == '`' {
			if inQuote {
				builder.WriteString("</code>")
			} else {
				builder.WriteString("<code>")
			}
			inQuote = !inQuote
		} else {

			builder.WriteRune(ch)

		}

	}

	return builder.String()

}

// converts a single changelog entry to html counterpart, accounting for newline sublists and codeblocks
func changeToHTML(s string) string {

	// requires LF line endings
	splits := strings.Split(s, "\n")

	builder := strings.Builder{}

	builder.WriteString("\t<li>" + codeQuoteHTML(splits[0]) + "</li>\n")

	if len(splits) == 1 {
		return builder.String()
	}

	builder.WriteString("\t<ul>\n")

	for _, v := range splits[1:] {

		if strings.Trim(v, " ") != "" {
			builder.WriteString("\t\t<li>" + codeQuoteHTML(strings.Trim(v, " ")) + "</li>\n")
		}

	}

	builder.WriteString("\t</ul>\n")

	return builder.String()

}

type changelog struct {
	Version map[string]string
	Changes map[string][]string
}

func (C changelog) assertVersionID() string {

	if s, ok := C.Version["id"]; ok {
		ids := strings.Split(s, ".")

		if len(ids) < 3 {
			log.Fatal("Version ", C, " Id is corrupted")
		}

		if len(ids) >= 4 {
			return ids[0] + "." + ids[1] + "." + ids[2] + "-" + ids[3]
		}
		return ids[0] + "." + ids[1] + "." + ids[2]

	}

	log.Fatal("Version ", C, " Has no Id")
	return ""
}

func (C changelog) toHTMLBuilder(buffer io.Writer) {

	writer := bufio.NewWriter(buffer)

	if name, ok := C.Version["name"]; ok {
		writer.WriteString(fmt.Sprintf("<h2>Sonnet V%s \"%s\"</h2>\n", C.assertVersionID(), name))
	} else {
		writer.WriteString(fmt.Sprintf("<h2>Sonnet V%s</h2>\n", C.assertVersionID()))
	}

	if note, ok := C.Version["note"]; ok {
		writer.WriteString(note)
		writer.WriteByte('\n')
	}

	changeMap := []struct{ name, prettyName string }{
		{"frontend", "Frontend"},
		{"backend", "Under the Hood"},
		{"runtime", "Runtime Environment"},
		{"bugs", "Bugs"},
	}

	for _, v := range changeMap {
		if changes, ok := C.Changes[v.name]; ok {

			writer.WriteString(fmt.Sprintf("<h3>%s:</h3>\n<ul>\n", v.prettyName))

			for _, change := range changes {

				writer.WriteString(changeToHTML(change))

			}

			writer.WriteString("</ul>\n")

		}
	}

	writer.Flush()

}

func (C changelog) toHTML() string {
	builder := new(strings.Builder)
	C.toHTMLBuilder(builder)
	return builder.String()
}

func generateChangelog() map[SemVer]changelog {

	files := unwrapExit(os.ReadDir(ChangelogDir))

	changelogs := map[SemVer]changelog{}

	for _, f := range files {

		if strings.HasSuffix(f.Name(), ".toml") && !f.IsDir() {

			chglog := new(changelog)

			_, err := toml.DecodeFile(ChangelogDir+f.Name(), chglog)
			if err != nil {
				log.Println(f.Name())
				log.Fatal(err)
			}

			changelogs[semVerFromString(strings.TrimSuffix(f.Name(), ".toml"))] = *chglog

		}

	}

	return changelogs

}

// SemVer is a semantic version consisting of 4 int's
type SemVer struct {
	Major int
	Minor int
	Micro int
	Nano  int
}

// A SemVerList is a list of SemVer with sort.Interface
type SemVerList []SemVer

func (S SemVerList) Len() int { return len(S) }
func (S SemVerList) Less(i, j int) bool {
	if S[i].Major == S[j].Major {
		if S[i].Minor == S[j].Minor {
			if S[i].Micro == S[j].Micro {
				return S[i].Nano < S[j].Nano
			}
			return S[i].Micro < S[j].Micro

		}
		return S[i].Minor < S[j].Minor

	}
	return S[i].Major < S[j].Major

}
func (S SemVerList) Swap(i, j int) { S[i], S[j] = S[j], S[i] }

func semVerFromString(s string) SemVer {

	frags := strings.Split(s, ".")

	if len(frags) < 3 {
		log.Fatal("Version ", s, " does not have enough fragments")
	}

	major := unwrapExit(strconv.Atoi(frags[0]))
	minor := unwrapExit(strconv.Atoi(frags[1]))
	micro := unwrapExit(strconv.Atoi(frags[2]))

	var nano int

	if len(frags) >= 4 {
		nano = unwrapExit(strconv.Atoi(frags[3]))
	}

	return SemVer{
		Major: major,
		Minor: minor,
		Micro: micro,
		Nano:  nano,
	}

}

func writeChangeLog() {

	changelog := generateChangelog()

	verlist := make([]SemVer, 0, len(changelog))
	for k := range changelog {
		verlist = append(verlist, k)
	}

	sort.Sort(sort.Reverse(SemVerList(verlist)))

	fp := unwrapExit(os.Create(HTMLDir + "changelog.b.html"))
	defer fp.Close()

	fp.WriteString("0\nChangelog - Sonnet\n<h1>Changelog</h1>\n")

	for _, k := range verlist {
		fp.WriteString("<div class=\"divider\"></div>\n")
		changelog[k].toHTMLBuilder(fp)
	}

}

func main() {

	writeChangeLog()

	buildTemplates()
}
