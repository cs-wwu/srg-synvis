package shader

type Shader struct {
	VertexShader   []byte
	FragmentShader []byte
}

// Compile the shader referenced by shaderPath
func CompileShader(shaderPath string) ([]byte, error) {
	// Read shader source (assets/shaders/shader.[vert|frag])
	// Compile shader
	// Return compiled shader as byte array
}

func LoadShader(vertPath, fragPath string) (*Shader, error) {
	vertShader, err := CompileShader(vertPath)
	if err != nil {
		return nil, err
	}
	fragShader, err := CompileShader(fragPath)
	if err != nil {
		return nil, err
	}
	return &Shader{
		VertexShader: vertShader,
		FragmentShader: fragShader,
	}, nil
}
