#version 450
#extension GL_ARB_separate_shader_objects : enable
#extension GL_ARB_shading_language_420pack : enable

layout(binding = 1) uniform sampler2D texSampler;

layout(location = 0) in vec3 fragColor;
layout(location = 1) in vec3 fragTexCoord;
layout(location = 2) in vec3 worldPos;

layout(location = 0) out vec4 outColor;

void main() {
    vec4 texColor = texture(texSampler, vec2(fragTexCoord.x, fragTexCoord.y));
    vec4 baseColor = texColor * vec4(fragColor, 1.0);
    
    // Check if the height (y-coordinate) is greater than 0 (then it's water)
    if (fragTexCoord.z > 0.5) {
        outColor = vec4(0.0, 0.0, 1.0, 1.0); // Blue color
    } else {
        outColor = baseColor;
    }
}
