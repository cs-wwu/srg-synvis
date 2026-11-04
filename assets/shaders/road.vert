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
    float viewportHeight;   // NEW: framebuffer height in pixels
} ubo;

layout(location = 0) in vec3 inPosition;    // road CENTERLINE point
layout(location = 1) in vec3 inColor;
layout(location = 2) in vec2 inNormal;      // unit sideways direction (x, z)
layout(location = 3) in vec2 inSideHalfW;   // x = side (-1 or +1), y = real half-width in world units

layout(location = 0) out vec3 fragColor;

const float MIN_PIXELS = 1.5;   // minimum on-screen road width

void main() {
  // World-space size of one pixel at this point's distance from the camera.
  vec4 clipCenter = ubo.proj * ubo.view * vec4(inPosition, 1.0);
  float w = max(clipCenter.w, 0.001);
  float pixelWorld = 2.0 * w / (abs(ubo.proj[1][1]) * ubo.viewportHeight);

  // Real width up close, never thinner than MIN_PIXELS far away.
  float halfW = max(inSideHalfW.y, 0.5 * MIN_PIXELS * pixelWorld);

  vec3 pos = inPosition + vec3(inNormal.x, 0.0, inNormal.y) * (inSideHalfW.x * halfW);
  gl_Position = ubo.proj * ubo.view * vec4(pos, 1.0);

  fragColor = inColor;
}