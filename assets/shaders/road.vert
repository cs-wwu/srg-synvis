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
    float viewportHeight;   // framebuffer height in pixels
} ubo;

layout(location = 0) in vec3 inPosition;    // road CENTERLINE point
layout(location = 1) in vec3 inColor;
layout(location = 2) in vec2 inNormal;      // unit sideways direction (x, z)
layout(location = 3) in vec2 inSideHalfW;   // x = side (-1 or +1), y = real half-width in world units

layout(location = 0) out vec3 fragColor;

const float MIN_PIXELS  = 1.5;  // minimum on-screen road thickness (perpendicular to the road, on screen)
const float LIFT_PIXELS = 3.0;  // raise road by this many pixels' worth of world height (distance-scaled)

void main() {
  mat4 vp = ubo.proj * ubo.view;

  vec4 c0 = vp * vec4(inPosition, 1.0);
  float w = max(c0.w, 0.001);
  float pixelWorld = 2.0 * w / (abs(ubo.proj[1][1]) * ubo.viewportHeight);

  // Distance-scaled lift: keeps the (widened) road above sloped terrain and
  // gives the depth test real separation at range.
  vec3 base = inPosition + vec3(0.0, LIFT_PIXELS * pixelWorld, 0.0);

  // Where the real-width edge would be.
  vec3 edge = base + vec3(inNormal.x, 0.0, inNormal.y) * (inSideHalfW.x * inSideHalfW.y);

  vec4 cB = vp * vec4(base, 1.0);
  vec4 cE = vp * vec4(edge, 1.0);

  // Viewport size in pixels. proj11/proj00 is the aspect ratio for both the
  // perspective and the map-view ortho projection.
  vec2 vpPx = vec2(ubo.viewportHeight * abs(ubo.proj[1][1] / ubo.proj[0][0]),
                   ubo.viewportHeight);

  // Signed on-screen offset (pixels) from centerline to the real-width edge.
  vec2 offPx = (cE.xy / max(cE.w, 0.001) - cB.xy / max(cB.w, 0.001)) * 0.5 * vpPx;
  float len = length(offPx);
  float minHalf = 0.5 * MIN_PIXELS;

  vec4 clip = cE;
  if (len < minHalf) {
    // Too thin on screen: enforce the minimum in screen space, keeping the
    // sign of the offset so triangle winding (back-face culling) is unchanged.
    vec2 dir = (len > 1e-4) ? offPx / len : vec2(inSideHalfW.x, 0.0);
    clip = cB;
    clip.xy += dir * (minHalf * 2.0) / vpPx * cB.w;
  }

  gl_Position = clip;
  fragColor = inColor;
}
