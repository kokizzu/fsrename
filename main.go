package main

import "flag"
import "os"
import "fmt"
import "log"
import "path/filepath"
import "regexp"
import "strings"
import "sync"
import "sort"

var matchPatternPtr = flag.String("match", ".", "regular expression without slash '/'")
var replacementPtr = flag.String("replace", "", "replacement")
var replacementFormatPtr = flag.String("replace-format", "", "replacement with format")
var fileOnlyPtr = flag.Bool("fileonly", false, "file only")
var dirOnlyPtr = flag.Bool("dironly", false, "directory only")
var forExtPtr = flag.String("forext", "", "extension name")
var dryRunPtr = flag.Bool("dryrun", false, "dry run only")
var numOfWorkersPtr = flag.Int("c", 2, "the number of concurrent rename workers. default = 2")
var trimPrefixPtr = flag.String("trimprefix", "", "trim prefix")
var trimSuffixPtr = flag.String("trimsuffix", "", "trim suffix")
var orderBy = flag.String("orderby", "", "order by")
var startNum = flag.Int("startnum", 0, "sequence number start with")
var sequence_number int = 0
var m sync.Mutex

type Entry struct {
	path    string
	newpath string
	info    os.FileInfo
}

func EntryPrinter(cv chan bool, input chan *Entry) {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for {
		entry, ok := <-input
		if entry == nil || !ok {
			break
		}

		if strings.HasPrefix(entry.path, pwd) {
			var oldpath = strings.TrimLeft(strings.Replace(entry.path, pwd, "", 1), "/")
			var newpath = strings.TrimLeft(strings.Replace(entry.path, pwd, "", 1), "/")
			fmt.Printf("%s => %s", oldpath, newpath)
		} else {
			fmt.Printf("%s => %s\n", entry.path, entry.newpath)
		}
	}
	cv <- true
}

func RenameWorker(cv chan bool, input chan *Entry, output chan *Entry, extRegExp *regexp.Regexp, matchRegExp *regexp.Regexp, replacement string, dryrun bool) {
	for {
		entry, ok := <-input
		if entry == nil || !ok {
			break
		}
		if extRegExp != nil && !extRegExp.MatchString(entry.info.Name()) {
			continue
		}
		if !matchRegExp.MatchString(entry.info.Name()) {
			continue
		}
		var replaceStr string
		if *replacementPtr != "" {
			replaceStr = *replacementPtr
		} else {
			replaceStr = *replacementFormatPtr
		}
		var newName = matchRegExp.ReplaceAllString(entry.info.Name(), replaceStr)
		// if seqNum {
		// 	extAddress := strings.LastIndex(newName, ".")
		// 	m.Lock()
		// 	newName = fmt.Sprintf("%s%03d.%s", newName[:extAddress], sequence_number, newName[extAddress+1:])
		// 	sequence_number = sequence_number + 1
		// 	m.Unlock()
		// }
		entry.newpath = filepath.Join(filepath.Dir(entry.path), newName)
		if !dryrun {
			os.Rename(entry.path, entry.newpath)
		}
		// fmt.Println("<-Entry newName:", newName)
		output <- entry
	}
	cv <- true
}

func main() {
	flag.Parse()
	var pathArgs = flag.Args()

	if len(pathArgs) == 0 {
		pathArgs = []string{"./"}
	}

	if *replacementFormatPtr != "" {
		*replacementPtr = ""
		*fileOnlyPtr = true
	}

	// Build pattern from prefix/suffix options
	if *trimPrefixPtr != "" {
		*matchPatternPtr = "^" + regexp.QuoteMeta(*trimPrefixPtr)
		*replacementPtr = ""
		*replacementFormatPtr = ""
	} else if *trimSuffixPtr != "" {
		*matchPatternPtr = regexp.QuoteMeta(*trimSuffixPtr) + "$"
		*replacementPtr = ""
		*replacementFormatPtr = ""
	}

	if *matchPatternPtr == "" {
		log.Fatalln("match pattern is required. use -match 'pattern'")
	}
	if *replacementPtr == "" && *replacementFormatPtr == "" {
		log.Fatalln("replacement is required. use -replace 'replacement' or -replace-format 'replacement with format'")
	}

	sequence_number = *startNum

	var matchRegExp = regexp.MustCompile(*matchPatternPtr)

	var extRegExp *regexp.Regexp = nil
	if *forExtPtr != "" {
		extRegExp = regexp.MustCompile("\\." + *forExtPtr + "$")
	}

	var numOfWorkers = *numOfWorkersPtr

	var workerCv = make(chan bool, numOfWorkers)
	var printerCv = make(chan bool)
	var entryQueue []Entry
	var entryOutput = make(chan *Entry, 1000)
	var renamedEntryOutput = make(chan *Entry, 1000)

	for i := 0; i < numOfWorkers; i++ {
		go RenameWorker(workerCv, entryOutput, renamedEntryOutput, extRegExp, matchRegExp, *replacementPtr, *dryRunPtr)
	}
	go EntryPrinter(printerCv, renamedEntryOutput)

	for _, pathArg := range pathArgs {
		matches, err := filepath.Glob(pathArg)
		if err != nil {
			panic(err)
		}

		for _, match := range matches {
			var err = filepath.Walk(match, func(path string, info os.FileInfo, err error) error {
				if *dirOnlyPtr {
					if info.IsDir() {
						entryQueue = append(entryQueue, Entry{path: path, info: info})
					}
				} else if *fileOnlyPtr {
					if !info.IsDir() {
						entryQueue = append(entryQueue, Entry{path: path, info: info})
					}
				} else {
					log.Println("Entry, ", path)
					entryQueue = append(entryQueue, Entry{path: path, info: info})
				}
				return err
			})
			if err != nil {
				panic(err)
			}
		}

		//Sorting.
		if *fileOnlyPtr {
			switch *orderBy {
			case "Reverse":
				sort.Sort(ReverseSort{entryQueue})
			case "Mtime":
				sort.Sort(MtimeSort{entryQueue})
			case "MtimeReverse":
				sort.Sort(MtimeReverseSort{entryQueue})
			case "Size":
				sort.Sort(SizeSort{entryQueue})
			case "SizeReverse":
				sort.Sort(SizeReverseSort{entryQueue})
			}
		}

		for _, v := range entryQueue {
			entryOutput <- &v
		}
	}
	entryOutput <- nil
	close(entryOutput)

	for ; numOfWorkers > 0; numOfWorkers-- {
		<-workerCv
	}
	close(workerCv)
	renamedEntryOutput <- nil

	<-printerCv
	close(printerCv)
}
