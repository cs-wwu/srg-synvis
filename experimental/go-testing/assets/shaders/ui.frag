#version 450
#extension GL_ARB_separate_shader_objects : enable
#extension GL_ARB_shading_language_420pack : enable
layout (binding = 0) uniform sampler2D tex[3];

layout (location = 0) in vec2 texcoord;
layout (location = 1) in flat int num;

layout (location = 0) out vec4 uFragColor;

void main() {
   uFragColor = texture(tex[2], texcoord);
}