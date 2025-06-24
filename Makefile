# Shader source and output directories
SHADER_DIR = assets/shaders
SHADER_SOURCES = $(wildcard $(SHADER_DIR)/*.vert) $(wildcard $(SHADER_DIR)/*.frag)
SHADER_OUTPUTS = $(SHADER_SOURCES:%=%.spv)

# Compiler settings
GLSLC = glslc
GO = go

.PHONY: run clean help

# Run the program
run: $(SHADER_OUTPUTS)
	$(GO) run ./cmd/svs

# Pattern rule for vertex shaders
$(SHADER_DIR)/%.vert.spv: $(SHADER_DIR)/%.vert
	$(GLSLC) $< -o $@

# Pattern rule for fragment shaders
$(SHADER_DIR)/%.frag.spv: $(SHADER_DIR)/%.frag
	$(GLSLC) $< -o $@

# Clean built shader files
clean:
	rm -f $(SHADER_OUTPUTS)
	$(GO) clean

# List all available targets
help:
	@echo "Available targets:"
	@echo "  run   - Build everything and run (default)"
	@echo "  clean - Remove compiled shader files"
	@echo "  help  - Show this help message"
