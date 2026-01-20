#version 450

layout(std140, binding = 0) uniform UBO {
  dmat4 model;
  dmat4 view;
  mat4 proj;
  mat4 normalM;
  dmat4 planeDirection;
  vec3 lightDir;
  float renderDistance;
  dvec3 cameraPos;
  float time;
  float disableWarnings;
  float disableFog;
} ubo;

layout(location = 0) in dvec3 inPosition; // NB: Takes up positions 0-1
layout(location = 2) in vec3 inColor;

layout(location = 0) out vec3 fragColor;

void main() {
  gl_Position = ubo.proj * vec4(ubo.view * dvec4(inPosition, 1.0));

  fragColor = inColor;
}
