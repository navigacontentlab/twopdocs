package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/navigacontentlab/twopdocs"
	"google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet

	var jsonFile string
	var specFile string
	var application string
	var version string
	var infomaker bool

	flags.StringVar(&application, "application", "", "the name of the application")
	flags.StringVar(&version, "version", "0.0.0", "the API version")
	flags.StringVar(&jsonFile, "json", "", "file to dump the JSON source to")
	flags.StringVar(&specFile, "file", "", "file to write the API spec to")
	flags.BoolVar(&infomaker, "infomaker", false, "support the infomaker.io domain")

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {

		gen.SupportedFeatures = internal_gengo.SupportedFeatures

		if application == "" {
			return errors.New("missing application name")
		}

		if specFile == "" {
			specFile = application + "-openapi.json"
		}

		doc := twopdocs.StructureDump(gen)

		if jsonFile != "" {
			enc := json.NewEncoder(gen.NewGeneratedFile(jsonFile, ""))
			enc.SetIndent("", "  ")

			err := enc.Encode(doc)
			if err != nil {
				return fmt.Errorf("failed to write out JSON docs: %w", err)
			}
		}

		sg := twopdocs.NewSchemaGenerator(doc)

		api, err := twopdocs.ToOpenAPI(application, version, doc, gen, sg)
		if err != nil {
			return fmt.Errorf("failed to render API spec: %w", err)
		}

		api.Servers = append(api.Servers,
			twopdocs.NavigaSaasServer(application, infomaker))

		f := gen.NewGeneratedFile(specFile, "")
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")

		err = enc.Encode(api)
		if err != nil {
			return fmt.Errorf("failed to marshal OpenAPI spec document: %w", err)
		}

		return nil
	})
}
