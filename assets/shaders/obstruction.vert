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

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec3 inColor;
layout(location = 2) in vec3 inNormal;

layout(location = 0) out vec3 fragColor;
layout(location = 1) out vec3 fragNormal;

void main() {
  // World position
  vec4 worldPos = ubo.model * vec4(inPosition, 1.0);
  gl_Position = ubo.proj * ubo.view * worldPos;

  fragColor = inColor;

  // Transform normal to world space
  fragNormal = normalize(mat3(ubo.normalM) * inNormal);
}