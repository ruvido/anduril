
package internal

import (
    "os"
    "time"
)

// getFileModTime fallback to file modification time
func getFileModTime(path string) (time.Time, error) {
    fi, err := os.Stat(path)
    if err != nil {
        return time.Time{}, err
    }
    return fi.ModTime(), nil
}
