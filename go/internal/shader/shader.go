package shader

import (
	"fmt"
	"os"

    "github.com/cs-wwu/srg-synvis/go/pkg/utils"

	gs "github.com/celer/gshaderc"
)

type ShaderKind int

const (
	VertexShader ShaderKind = iota
)

func CompileShader(filename string, kind ShaderKind) ([]byte, error) {
	options := gs.NewCompilerOptions()
	defer options.Release()
	compiler := gs.NewCompiler()
	defer compiler.Release()
	data, err := os.ReadFile(filename)
	utils.orPanic(err)

	result := compiler.CompileIntoSPV(string(data), shaderType, filename, entryPoint, optinos)
	utils.orPanic(result.Error()) // TODO not sure if this works as inteded
	defer result.Release()
}

// LoadShader loads and compiles both vertex and fragment shaders
func LoadShader(vertexShaderPath, fragmentShaderPath string) ([]byte, []byte, error) {
	compiler := gs.NewCompiler()
	defer compiler.Release()

	vertexShader, err := compiler.CompileIntoSPV()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compile vertex shader: %v", err)
	}

	fragmentShader, err := compiler.CompileIntoSPV()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compile fragment shader: %v", err)
	}

	return vertexShader, fragmentShader, nil
}
