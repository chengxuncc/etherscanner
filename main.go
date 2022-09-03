package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	host   = flag.String("host", "api.etherscan.io", "etherscan api host")
	apiKey = flag.String("api-key", "CIVXDMVSH6KIKN3YCWQX6TJMK7U9Y8HDJU", "etherscan API KEY")
	dir    = flag.String("dir", "", "directory to save")
)

func main() {
	flag.Parse()
	for {
		var address string
		fmt.Print("Please enter contract address: ")
		_, err := fmt.Scan(&address)
		if err != nil {
			return
		}
		address = strings.ToLower(address)
		if !strings.HasPrefix(address, "0x") {
			address = "0x" + address
		}
		downloadSourceCode(address)
	}
}

func downloadSourceCode(address string) {
	url := fmt.Sprintf(`https://%s/api?module=contract&action=getsourcecode&apikey=%s&address=%s`, *host, *apiKey, address)
	fmt.Println("request", url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("request etherscan error", err)
		return
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	var result EtherscanResult
	err = decoder.Decode(&result)
	if err != nil {
		b, _ := io.ReadAll(resp.Body)
		fmt.Println("decode source code result error:", err, "body:", string(b))
		return
	}
	if result.Message != "OK" {
		b, err := json.Marshal(result)
		if err != nil {
			panic(err)
		}
		fmt.Println("etherscan error:", string(b))
		return
	}
	var sourceCodeResults []SourceCodeResult
	err = json.Unmarshal(result.Result, &sourceCodeResults)
	if err != nil {
		fmt.Println("unmarshal source code result error:", err)
		return
	}
	sourceCodeResult := sourceCodeResults[0]
	if sourceCodeResult.SourceCode == "" {
		fmt.Println(address, sourceCodeResult.ABI)
		return
	}
	saveDir := path.Join(*dir, address)
	err = saveSourceCode(saveDir, sourceCodeResult)
	if err != nil {
		fmt.Println("save source code error:", err)
		return
	}
	absSaveDir, err := filepath.Abs(saveDir)
	if err != nil {
		panic(err)
	}
	fmt.Println(address, "contract source code saved to", absSaveDir)
}

func saveSourceCode(dir string, result SourceCodeResult) error {
	name := result.ContractName
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	err = saveJsonFile(path.Join(dir, name), result)
	if err != nil {
		return err
	}
	if result.ABI != "" {
		abiBytes := []byte(result.ABI)
		var prettyJSON bytes.Buffer
		if e := json.Indent(&prettyJSON, abiBytes, "", "  "); e == nil {
			abiBytes = prettyJSON.Bytes()
		}
		err = saveFile(path.Join(dir, name+"-abi.json"), string(abiBytes))
		if err != nil {
			return err
		}
	}
	switch result.SourceCode[0] {
	case '{':
		source := result.SourceCode
		if len(source) > 4 && source[:2] == "{{" && source[len(source)-2:] == "}}" {
			// unwrap invalid json from uploaded standard json source code
			source = source[1 : len(source)-1]
		}
		var sss StandardJSONSourceCode
		err = json.Unmarshal([]byte(source), &sss)
		if err != nil {
			return err
		}
		for solName, code := range sss.Sources {
			err = saveFile(path.Join(dir, solName+".sol"), code.Content)
			if err != nil {
				return err
			}
		}
	default:
		err = saveFile(path.Join(dir, name+".sol"), result.SourceCode)
		if err != nil {
			return err
		}
	}
	return nil
}

func saveJsonFile(name string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(name+".json", b, 0666)
}

func saveFile(name, content string) error {
	err := os.MkdirAll(path.Dir(name), 0755)
	if err != nil {
		return err
	}
	return os.WriteFile(name, []byte(content), 0666)
}

type EtherscanResult struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

type SourceCodeResult struct {
	ContractName         string `json:"ContractName"`
	CompilerVersion      string `json:"CompilerVersion"`
	OptimizationUsed     string `json:"OptimizationUsed"`
	Runs                 string `json:"Runs"`
	EVMVersion           string `json:"EVMVersion"`
	Library              string `json:"Library"`
	LicenseType          string `json:"LicenseType"`
	Proxy                string `json:"Proxy"`
	Implementation       string `json:"Implementation"`
	SwarmSource          string `json:"SwarmSource"`
	ConstructorArguments string `json:"ConstructorArguments"`
	ABI                  string `json:"ABI"`
	SourceCode           string `json:"SourceCode"`
}

type StandardJSONSourceCode struct {
	Language string `json:"language"`
	Sources  map[string]struct {
		Content string `json:"content"`
	} `json:"sources"`
	Settings struct {
		Optimizer struct {
			Enabled bool `json:"enabled"`
			Runs    int  `json:"runs"`
		} `json:"optimizer"`
		OutputSelection struct {
			Field1 struct {
				Field1 []string `json:"*"`
			} `json:"*"`
		} `json:"outputSelection"`
		Metadata struct {
			UseLiteralContent bool `json:"useLiteralContent"`
		} `json:"metadata"`
		Libraries struct {
		} `json:"libraries"`
	} `json:"settings"`
}
