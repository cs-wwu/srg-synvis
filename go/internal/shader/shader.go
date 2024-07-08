// Package shader provides compiling and processing of shader files
package shader

import (
	"fmt"
	"os"

	gs "github.com/celer/gshaderc"
)

// Compiles the shader specified by filename, and its type
// It saves the shader to the filepath with .spv on the end
func CompileShader(filepath string, shaderType gs.ShaderType) error {
	// Generate a shader compiler
	options := gs.NewCompilerOptions()
	defer options.Release()
	compiler := gs.NewCompiler()
	defer compiler.Release()
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read shader file %s: %v", filepath, err)
	}

	// Compile the shader specified by filename to an SPV
	entryPoint := "main" // TODO may need to change
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
	// Vertex Shader
	vertShaderType := gs.ShaderType(gs.VertexShader) //	Pull Shader Type
	err := CompileShader(vertexShaderPath, vertShaderType)
	if err != nil {
		return fmt.Errorf("failed to compile vertex shader: %v", err)
	}

	// Fragment Shader
	fragShaderType := gs.ShaderType(gs.FragmentShader) // Pull Shader Type
	err = CompileShader(fragmentShaderPath, fragShaderType)
	if err != nil {
		return fmt.Errorf("failed to compile fragment shader: %v", err)
	}

	return nil
}
