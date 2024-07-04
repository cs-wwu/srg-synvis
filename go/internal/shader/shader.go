package shader

import (
	"fmt"
	"os"

	gs "github.com/celer/gshaderc"
)

// Compiles the shader specified by filename, and its type
// It saves the shader to the filepath with .spv on the end
func CompileShader(filepath string, shaderType gs.ShaderType) error {
	options := gs.NewCompilerOptions()
	defer options.Release()
	compiler := gs.NewCompiler()
	defer compiler.Release()
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read shader file %s: %v", filepath, err)
	}

	entryPoint := "main"
	result := compiler.CompileIntoSPV(string(data), shaderType, filepath, entryPoint, options)
	if result.Error() != nil {
		return fmt.Errorf("failed to compile shader %s: %v", filepath, result.Error())
	}
	defer result.Release()

	// Write the compiled shader to a new file
	outputFilename := filepath + ".spv"
	err = os.WriteFile(outputFilename, result.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write compiled shader to file %s: %v", outputFilename, err)
	}

	return nil
}

// Compiles both the fragment and vertex shaders pointed to by the filepaths
func CompileShaders(vertexShaderPath, fragmentShaderPath string) error {
	err := CompileShader(vertexShaderPath, gs.VertexShader)
	if err != nil {
		return fmt.Errorf("failed to compile vertex shader: %v", err)
	}

	err = CompileShader(fragmentShaderPath, gs.FragmentShader)
	if err != nil {
		return fmt.Errorf("failed to compile fragment shader: %v", err)
	}

	return nil
}
