// Copyright 2021 the Exposure Notifications Verification Server authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]

	for _, fName := range args {
		fmt.Printf("processing: %q\n", fName)
		input, err := os.ReadFile(fName)
		if err != nil {
			panic(err)
		}

		contents := string(input)
		lines := strings.Split(contents, "\n")

		outputLines := make([]string, 0)
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			line = strings.ReplaceAll(line, "«", "\"")
			line = strings.ReplaceAll(line, "»", "\"")
			line = strings.TrimSuffix(line, ".")

			if strings.Contains(line, "msgstr") && i > 2 {
				line = fmt.Sprintf("%s\n", line)
			}
			outputLines = append(outputLines, line)

			if strings.Contains(line, "Plural-Forms") {
				outputLines = append(outputLines, "\n")
			}
			if strings.Contains(line, "# -") {
				outputLines = append(outputLines, "\n")
			}
		}

		info, err := os.Stat(fName)
		if err != nil {
			panic(err)
		}

		output := strings.Join(outputLines, "\n")
		os.WriteFile(fName, []byte(output), info.Mode())
	}
}
