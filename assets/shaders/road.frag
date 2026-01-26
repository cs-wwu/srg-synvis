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

layout(location = 0) in vec3 fragColor;

layout(location = 0) out vec4 outColor;

void main() {
  outColor = vec4(fragColor, 1);
}
