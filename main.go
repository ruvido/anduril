
package main

import (
    "anduril/cmd"
    "log"
)

func main() {
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}
