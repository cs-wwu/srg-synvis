#version 450
#extension GL_ARB_separate_shader_objects : enable

layout(std140, binding = 0) uniform UBO {
    mat4 model;
    mat4 view;
    mat4 proj;
    mat4 normalM;
    vec3 lightDir;
    float time;
} ubo;

layout(location = 0) in vec3 inTexCoord;
layout(location = 0) out vec4 outColor;

// Procedural sky function - no texture needed
vec4 getSkyColor(vec3 direction) {
    // Normalize direction
    vec3 dir = normalize(direction);
    
    // Blue zenith
    vec3 zenithColor = vec3(0.1, 0.3, 0.8);
    
    // Light blue horizon
    vec3 horizonColor = vec3(0.7, 0.85, 1.0);
    
    // Interpolate based on y-component (up direction)
    float t = max(dir.y, 0.0);
    
    // Apply non-linear gradient for more dramatic effect
    float skyGradient = pow(t, 0.4);
    
    return vec4(mix(horizonColor, zenithColor, skyGradient), 1.0);
}

void main() {
    outColor = getSkyColor(inTexCoord);
}
