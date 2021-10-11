package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	templateroot = "./builders/"
	htmlroot = "./html/"
)

type Walkstruct map[string]string

func (H Walkstruct) walkerfunc(fpath string, info fs.FileInfo, err error) error {
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

func transform(filePath, fileData, temp string, rch chan int) {
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

		newfile, ferr := os.Create(filePath[:len(filePath)-6] + "html")
		defer newfile.Close()
		if ferr != nil {
			log.Fatal(ferr)
		}
		_, werr := newfile.Write([]byte(nextemp))
		if werr != nil {
			log.Fatal(werr)
		}
	}
	// Tell the main thread we are done
	rch <- 0
}

func main() {

	templates := make(map[string]string)
	tfiles, direrr := os.ReadDir(templateroot + "templatedata")

	if direrr != nil {
		log.Fatal(direrr)
	}

	for _, f := range tfiles {
		fname := f.Name()
		if strings.HasSuffix(fname, ".t.html") {
			cont, readerr := os.ReadFile(templateroot + "templatedata/" + fname)
			if readerr != nil {
				fmt.Println("Error reading file:", fname)
			} else {
				templates[fname[:len(fname)-7]] = string(cont)
			}
		}
	}

	ConvFiles := Walkstruct{}
	walkerr := filepath.Walk(htmlroot, ConvFiles.walkerfunc)

	if walkerr != nil {
		log.Fatal(walkerr)
	}

	templatefile, err := os.ReadFile(templateroot + "TEMPLATE.t.html")

	if err != nil {
		log.Fatal(err)
	}

	rawstart := string(templatefile)

	rch := make(chan int)

	// Add templatedata files
	replacements := make([]string, 0, len(templates)*2)
	for k, v := range templates {
		ev := strings.Join(strings.Split(v, "\n"), "\n\t\t\t")
		replacements = append(replacements, "<TEMPLATE>["+k+"]</TEMPLATE>", ev)
	}

	rawstart = strings.NewReplacer(replacements...).Replace(rawstart)

	for path, data := range ConvFiles {
		go transform(path, data, rawstart, rch)
	}

	// wait for threads to finish
	for range ConvFiles {
		<-rch
	}
}
