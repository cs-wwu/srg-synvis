# Shader source and output directories
SHADER_DIR = assets/shaders
SHADER_SOURCES = $(wildcard $(SHADER_DIR)/*.vert) $(wildcard $(SHADER_DIR)/*.frag)
SHADER_OUTPUTS = $(SHADER_SOURCES:%=%.spv)

# Compiler settings
GLSLC = glslc
GO = go

# Default target
all: shaders

# Compile shaders
shaders: $(SHADER_OUTPUTS)

# Pattern rule for vertex shaders
$(SHADER_DIR)/%.vert.spv: $(SHADER_DIR)/%.vert
	$(GLSLC) $< -o $@

# Pattern rule for fragment shaders
$(SHADER_DIR)/%.frag.spv: $(SHADER_DIR)/%.frag
	$(GLSLC) $< -o $@

# Clean built shader files
clean:
	rm -f $(SHADER_OUTPUTS)

# Run the program
run:
	$(GO) run cmd/svs/main.go

# Build and run
build-run: shaders run

# List all available targets
help:
	@echo "Available targets:"
	@echo "  all        - Build shaders (default target)"
	@echo "  shaders    - Compile all shaders to SPIR-V"
	@echo "  clean      - Remove compiled shader files"
	@echo "  run        - Run the program"
	@echo "  build-run  - Compile shaders and run the program"
	@echo "  help       - Show this help message"

.PHONY: all shaders clean run build-run help