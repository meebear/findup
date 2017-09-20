package main

import (
    "fmt"
    "strings"
    "path/filepath"
    "os"
    "io"
    "io/ioutil"
    "sync"
    "time"
    "crypto/md5"
    "github.com/dustin/go-humanize"
)

type fileMeta struct {
    fullpath, basename string
    size    int64
    hash    string
}

func (fm *fileMeta) getKeyString() string {
    keys := []string{fm.basename, fmt.Sprintf("%d", fm.size), fm.hash}
    return strings.Join(keys, ":")
}

type fileMap map[string][]*fileMeta

var wg sync.WaitGroup
var sem = make(chan struct{}, 20) //number of files opened concurrently

func main() {
    parseCmdLine()

    start := time.Now()

    var filesCh = make(chan *fileMeta)
    for _, dir := range mydirs {
        wg.Add(1)
        go walkDir(dir, filesCh)
    }

    var allFiles = make(fileMap)
    var totalSize int64
    var numFiles int64
    var wait1 = make(chan struct{})
    var wait2 = make(chan struct{})

    // collector routine
    go func() {
        defer func() { wait1 <- struct{}{} }()
        for fm := range filesCh {
            numFiles++
            totalSize += fm.size
            key := fm.getKeyString()
            //fmt.Printf("%s\n", key)
            allFiles[key] = append(allFiles[key], fm)
        }
    }()

    // status update routine
    go func() {
        tick := time.NewTicker(time.Millisecond * 250)
        defer tick.Stop()
        var done = false
        for {
            select {
            case <-tick.C:
                fmt.Printf("\r%d files of total %s bytes", numFiles, humanize.Bytes(uint64(totalSize)))
            case <-wait1:
                done = true
            }

            if done {
                wait2 <- struct{}{} // inform main routine to go on
                break
            }
        }
    }()

    wg.Wait()
    close(filesCh) // inform collector routine to stop

    <-wait2
    end := time.Now()

    var nDupFiles int64
    var nDupBytes int64

    for _, fms := range allFiles {
        if len(fms) > 1 {
            nDupFiles += int64(len(fms) - 1)
            nDupBytes += fms[0].size * int64(len(fms) - 1)
            if *verbose {
                fmt.Printf("Dup: (%s)\n", humanize.Bytes(uint64(fms[0].size)))
                for _, fm := range fms {
                    fmt.Printf("  %s\n", fm.fullpath)
                }
            }
        }
    }

    fmt.Printf("\rScanned %d files of total %s bytes\n", numFiles, humanize.Bytes(uint64(totalSize)))
    fmt.Printf("\n%d files (%s) are duplicated\n", nDupFiles, humanize.Bytes(uint64(nDupBytes)))
    fmt.Printf("\nDone. used %v\n", end.Sub(start))
}

func walkDir(dir string, filesCh chan<- *fileMeta) {
    sem <- struct{}{}
    defer func() {
        <-sem
        wg.Done()
    }()

    for _, entry := range dirents(dir) {
        fpath := filepath.Join(dir, entry.Name())
        if entry.IsDir() {
            if entry.Name() != ".git" {
                wg.Add(1)
                go walkDir(fpath, filesCh)
            }
        } else {
            fm, err := getFileMeta(fpath, entry)
            if err != nil {
                fmt.Fprintf(os.Stderr, "err: %v\n", err)
            } else {
                filesCh <- fm
            }
        }
    }
}

func dirents(dir string) []os.FileInfo {
    entries, err := ioutil.ReadDir(dir)
    if err != nil {
        fmt.Fprintf(os.Stderr, "err: %v\n", err)
        return nil
    }
    return entries
}

func getFileMeta(fpath string, fi os.FileInfo) (*fileMeta, error) {
    var fm fileMeta
    fm.fullpath = fpath
    fm.basename = fi.Name()
    fm.size = fi.Size()
    n := fm.size
    if n > 512 * 1024 {
        n = 512 * 1024
    }

    f, err := os.Open(fpath)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    h := md5.New()
    if _, err := io.CopyN(h, f, n); err != nil {
        return nil, err
    }
    fm.hash = fmt.Sprintf("%x", h.Sum(nil))

    return &fm, nil
}
