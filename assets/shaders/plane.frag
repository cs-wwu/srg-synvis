#version 450

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

layout(location = 0) in vec3 fragColor;
layout(location = 1) in vec2 fragTexCoord;
layout(location = 2) in vec3 fragNormal;
layout(location = 3) in vec3 worldPos;

layout(location = 0) out vec4 outColor;

void main() {
    outColor = vec4(fragColor, 1);
}
