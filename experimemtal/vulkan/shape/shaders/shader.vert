#version 450

layout(binding = 0) uniform UniformBufferObject {
    mat4 model;
    mat4 view;
    mat4 proj;
} ubo;

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec3 inColor;
layout(location = 2) in vec3 inNormal;
layout(location = 3) in vec2 inTexCoord;

const vec3 DIRECTION_TO_LIGHT = normalize(vec3(0.0, 1.0, 0.0));
const float AMBIENT = 0.04;

layout(location = 0) out vec3 fragColor;
layout(location = 1) out vec2 fragTexCoord;

void main() {
    gl_Position = ubo.proj * ubo.view * ubo.model * vec4(inPosition, 1.0);

    vec3 world = normalize(mat3(ubo.model) * inNormal);
    float intense = AMBIENT + max(dot(world, DIRECTION_TO_LIGHT), 0);

    fragColor = intense * inColor;
    fragTexCoord = inTexCoord;
}
