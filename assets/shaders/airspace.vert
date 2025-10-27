#version 450

layout(binding = 0) uniform UniformBufferObject {
    mat4 model;
    mat4 view;
    mat4 projection;
    // ... other uniforms
} ubo;

layout(location = 0) in dvec3 inPosition;  // double precision for vec3.T
layout(location = 1) in vec4 inColor;

layout(location = 0) out vec4 fragColor;

void main() {
    fragColor = inColor;
}