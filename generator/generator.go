package generator

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	log "github.com/sirupsen/logrus" // nolint: depguard

	"github.com/dpup/protoc-gen-grpc-gateway-ts/registry"
	"github.com/pkg/errors"
)

// TypeScriptGRPCGatewayGenerator is the protobuf generator for typescript
type TypeScriptGRPCGatewayGenerator struct {
	Registry *registry.Registry
}

// New returns an initialised generator
func New(reg *registry.Registry) (*TypeScriptGRPCGatewayGenerator, error) {
	return &TypeScriptGRPCGatewayGenerator{
		Registry: reg,
	}, nil
}

// Generate take a code generator request and returns a response. it analyse request with registry and use the generated data to render ts files
func (t *TypeScriptGRPCGatewayGenerator) Generate(req *plugin.CodeGeneratorRequest) (*plugin.CodeGeneratorResponse, error) {
	resp := &plugin.CodeGeneratorResponse{}

	filesData, err := t.Registry.Analyse(req)
	if err != nil {
		return nil, errors.Wrap(err, "error analysing proto files")
	}
	tmpl := GetTemplate(t.Registry)
	log.Debugf("files to generate %v", req.GetFileToGenerate())

	needToGenerateFetchModule := false
	// feed fileData into rendering process
	for _, fileData := range filesData {
		if !t.Registry.IsFileToGenerate(fileData.Name) {
			log.Debugf("file %s is not the file to generate, skipping", fileData.Name)
			continue
		}

		log.Debugf("generating file for %s", fileData.TSFileName)
		data := &templateData{
			File:               fileData,
			EnableStylingCheck: t.Registry.EnableStylingCheck,
			UseStaticClasses:   t.Registry.UseStaticClasses,
		}
		generated, err := t.generateFile(data, tmpl)
		if err != nil {
			return nil, errors.Wrap(err, "error generating file")
		}
		resp.File = append(resp.File, generated)
		needToGenerateFetchModule = needToGenerateFetchModule || fileData.Services.NeedsFetchModule()
	}

	if needToGenerateFetchModule {
		// generate fetch module
		fetchTmpl := GetFetchModuleTemplate()
		log.Debugf("generate fetch template")
		generatedFetch, err := t.generateFetchModule(fetchTmpl)
		if err != nil {
			return nil, errors.Wrap(err, "error generating fetch module")
		}

		resp.File = append(resp.File, generatedFetch)
	}

	return resp, nil
}

func (t *TypeScriptGRPCGatewayGenerator) generateFile(data *templateData, tmpl *template.Template) (*plugin.CodeGeneratorResponse_File, error) {
	w := bytes.NewBufferString("")

	if data.IsEmpty() {
		w.Write([]byte(fmt.Sprintln("export default {}")))
	} else {
		err := tmpl.Execute(w, data)
		if err != nil {
			return nil, errors.Wrapf(err, "error generating ts file for %s", data.Name)
		}
	}

	fileName := data.TSFileName
	content := strings.TrimSpace(w.String())

	return &plugin.CodeGeneratorResponse_File{
		Name:           &fileName,
		InsertionPoint: nil,
		Content:        &content,
	}, nil
}

func (t *TypeScriptGRPCGatewayGenerator) generateFetchModule(tmpl *template.Template) (*plugin.CodeGeneratorResponse_File, error) {
	w := bytes.NewBufferString("")
	fileName := filepath.Join(t.Registry.FetchModuleDirectory, t.Registry.FetchModuleFilename)
	err := tmpl.Execute(w, &templateData{
		EnableStylingCheck: t.Registry.EnableStylingCheck,
		UseStaticClasses:   t.Registry.UseStaticClasses,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error generating fetch module at %s", fileName)
	}

	content := strings.TrimSpace(w.String())
	return &plugin.CodeGeneratorResponse_File{
		Name:           &fileName,
		InsertionPoint: nil,
		Content:        &content,
	}, nil
}
