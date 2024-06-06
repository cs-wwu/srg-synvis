#version 450

layout(binding = 0) uniform UniformBufferObject {
    mat4 model;
    mat4 view;
    mat4 proj;
    mat4 normalM;
    vec3 lightDir;
} ubo;

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec3 inColor;
layout(location = 2) in vec3 inNormal;
layout(location = 3) in vec2 inTexCoord;

const float AMBIENT = 0.04;

layout(location = 0) out vec3 fragColor;
layout(location = 1) out vec2 fragTexCoord;

void main() {
    gl_Position = ubo.proj * ubo.view * ubo.model * vec4(inPosition, 1.0);

    vec3 world = normalize(mat3(ubo.normalM) * inNormal);
    float intense = AMBIENT + max(dot(world, ubo.lightDir), 0);

    fragColor = intense * inColor;
    fragTexCoord = inTexCoord;
}
