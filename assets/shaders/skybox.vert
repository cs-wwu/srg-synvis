#version 450
#extension GL_ARB_separate_shader_objects : enable

layout(std140, binding = 0) uniform UBO {
    mat4 model;
    mat4 view;
    mat4 proj;
    mat4 normalM;
    mat4 planeDirection;
    vec3 lightDir;
    float renderDistance;
    vec3 cameraPos;
    float time;
    float disableWarnings;
    float disableFog;
} ubo;

layout(location = 0) in vec3 inPosition;
layout(location = 0) out vec3 outTexCoord;

void main() {
    // Use position as texture coordinate
    outTexCoord = inPosition;
    
    // Remove translation from view matrix to keep skybox centered on camera
    mat4 viewNoTranslation = ubo.view;
    viewNoTranslation[3].xyz = vec3(0.0, 0.0, 0.0);
    
    // Use projection and modified view matrix
    gl_Position = ubo.proj * viewNoTranslation * vec4(inPosition, 1.0);
    
    // Force z to be at the far plane (w) for depth testing
    gl_Position = gl_Position.xyww;
}
