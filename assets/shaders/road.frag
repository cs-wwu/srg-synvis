#version 450

layout(std140, binding = 0) uniform UBO {
    mat4 model;
    mat4 view;
    mat4 proj;
    mat4 normalM;
    mat4 planeDirection;
    vec4 overlayColor;
    vec3 lightDir;
    float renderDistance;
    vec3 cameraPos;
    float time;
    float disableWarnings;
    float disableFog;
} ubo;

layout(location = 0) in vec3 fragColor;

layout(location = 0) out vec4 outColor;

void main() {
  outColor = vec4(fragColor, 1);
}
