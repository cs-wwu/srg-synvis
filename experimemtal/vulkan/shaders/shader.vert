#version 450

layout(binding = 0) uniform UniformBufferObject {
    mat4 model;
    mat4 view;
    mat4 proj;
    mat4 normalM;

    vec4 ambient;
    vec3 lightPos;
    vec4 lightColor;
} ubo;

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec3 inColor;
layout(location = 2) in vec3 inNormal;
layout(location = 3) in vec2 inTexCoord;


layout(location = 0) out vec3 fragColor;
layout(location = 1) out vec2 fragTexCoord;

void main() {
    vec4 positionWorld = ubo.model * vec4(inPosition, 1.0);
    gl_Position = ubo.proj * ubo.view * positionWorld;

    vec3 normalWorld = normalize(mat3(ubo.normalM) * inNormal);

    vec3 directiontoLight = ubo.lightPos - positionWorld.xyz;
    float attenuation = 1.0 / dot(directiontoLight, directiontoLight);  // distance squared

    vec3 lcolor = ubo.lightColor.xyz * ubo.lightColor.w * attenuation;
    vec3 alight = ubo.ambient.xyz * ubo.ambient.w;
    vec3 diffuse = lcolor * max(dot(normalWorld, normalize(directiontoLight)), 0);

    fragColor = (diffuse + alight) * inColor;
    fragTexCoord = inTexCoord;
}
