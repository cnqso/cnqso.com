// library-password reads a password from stdin; plaintext never appears in arguments or output.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"server/config"
	"server/internal/libraryauth"
	"strings"
)

func main() {
	password, err := bufio.NewReader(io.LimitReader(os.Stdin, 74)).ReadString('\n')
	if err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, "Could not read password.")
		os.Exit(1)
	}
	if err := libraryauth.SetPassword(config.LibraryDir, []byte(strings.TrimSuffix(password, "\n"))); err != nil {
		fmt.Fprintln(os.Stderr, "Could not set library password:", err)
		os.Exit(1)
	}
	fmt.Println("Library password updated. Existing sessions are signed out.")
}
