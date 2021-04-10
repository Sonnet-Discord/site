package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var templateroot string = "./builders/"
var htmlroot string = "./html/"

func endswith(data, end string) bool {
	if len(end) > len(data) {
		return false
	}
	return data[len(data)-len(end):] == end
}

type AbsoluteFile struct {
	Path string
	Data string
}

var ConvFiles []AbsoluteFile = *new([]AbsoluteFile)

func walkerfunc(fpath string, info fs.FileInfo, err error) error {

	fname := info.Name()
	if endswith(fname, ".b.html") && !info.IsDir() {
		cont, err := ioutil.ReadFile(fpath)
		if err == nil {
			ConvFiles = append(ConvFiles, AbsoluteFile{fpath, string(cont)})
		}
	}
	return nil
}

func transform(file AbsoluteFile, nextemp string, tlates map[string]string, rch chan int) {
	dat := strings.Split(file.Data, "\n")
	if dat[0] == "0" {
		title := dat[1]
		content := strings.Join(dat[2:], "\n\t\t\t")
		// Add templatedata files
		for k, v := range tlates {
			ev := strings.Join(strings.Split(v, "\n"), "\n\t\t\t")
			nextemp = strings.Replace(nextemp, "<TEMPLATE>["+k+"]</TEMPLATE>", ev, 1)
		}
		// Add title and content
		nextemp = strings.Replace(nextemp, "<TEMPLATE>[HTML-TITLE]</TEMPLATE>", title, 1)
		nextemp = strings.Replace(nextemp, "<TEMPLATE>[HTML-CONTENT]</TEMPLATE>", content, 1)
		newfile, ferr := os.Create(file.Path[:len(file.Path)-6] + "html")
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
	tfiles, direrr := ioutil.ReadDir(templateroot + "templatedata")

	if direrr != nil {
		log.Fatal(direrr)
	}

	for _, f := range tfiles {
		fname := f.Name()
		if endswith(fname, ".t.html") {
			cont, readerr := ioutil.ReadFile(templateroot + "templatedata/" + fname)
			if readerr != nil {
				fmt.Println("Error reading file:", fname)
			} else {
				templates[fname[:len(fname)-7]] = string(cont)
			}
		}
	}

	walkerr := filepath.Walk(htmlroot, walkerfunc)

	if walkerr != nil {
		log.Fatal(walkerr)
	}

	templatefile, err := ioutil.ReadFile(templateroot + "TEMPLATE.t.html")

	if err != nil {
		log.Fatal(err)
	}

	rawstart := string(templatefile)

	rch := make(chan int)

	for _, file := range ConvFiles {
		go transform(file, rawstart, templates, rch)
	}
	for i := 0; i < len(ConvFiles); i++ {
		<-rch
	}
}
