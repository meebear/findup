package main

import (
    "fmt"
    "strings"
    "flag"
)

type fpaths []string

func (ps *fpaths) String() string {
    return fmt.Sprintf("%v", *ps)
}

func (ps *fpaths) Set(v string) error {
    p := strings.Split(v, ",")
    for _, s := range p {
        *ps = append(*ps, s)
    }
    return nil
}

type cliOptions struct {
    types *string
    saveto *string
    keyops *string
    verbose *bool
    mydirs fpaths
}

var cliops = cliOptions{
    saveto: flag.String("s", "", "save list of duplicated files to a file"),
    types: flag.String("t", "", "file types for checking, seperated by ','"),
    keyops: flag.String("o", "",
                    `compare options seperated by ','
            Options:
              name  file basename (included by default)
              size  file size (included by default)
              hash  md5 hash of the file`),
    verbose: flag.Bool("v", false, "show list of duplicated files"),
}

func parseCmdLine() error {
    flag.Var(&cliops.mydirs, "d", "directory to scan")
    flag.Parse()

    if len(cliops.mydirs) == 0 {
        cliops.mydirs = fpaths{"."}
    }
    return nil
}
