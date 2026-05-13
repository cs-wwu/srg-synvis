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
layout(location = 1) in vec3 fragNormal;

layout(location = 0) out vec4 outColor;

void main() {
  vec3 N = normalize(fragNormal);
  //vec3 L = normalize(ubo.lightDir);
  vec3 L = normalize(vec3(1.0, 1.0, 1.0));

  const float AMBIENT = 0.5;
  //float diffuse = max(dot(N, L), 0.0);
  float diffuse = dot(N, L) * 0.5 + 0.5;
  diffuse = clamp(diffuse, 0.0, 1.0);

  float lighting = AMBIENT + diffuse * (1.0 - AMBIENT);

  vec3 finalColor = fragColor * lighting;

  outColor = vec4(finalColor, 1.0);
}