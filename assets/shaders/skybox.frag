#version 450
#extension GL_ARB_separate_shader_objects : enable

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

layout(location = 0) in vec3 inTexCoord;
layout(location = 0) out vec4 outColor;

// Enhanced procedural sky with sun and atmospheric haze
vec4 getSkyColor(vec3 direction) {
    // Normalize direction
    vec3 dir = normalize(direction);
    vec3 lightDir = normalize(ubo.lightDir);
    
    // Deeper, more saturated sky colors
    vec3 zenithColor = vec3(0.05, 0.15, 0.5);        // Deep blue zenith
    vec3 horizonColor = vec3(0.8, 0.9, 0.95);        // Foggy white horizon
    vec3 sunColor = vec3(1.0, 0.95, 0.8);            // Warm sun color
    vec3 sunHaloColor = vec3(1.0, 0.8, 0.6);         // Orange halo around sun
    
    // Calculate sky gradient - more atmospheric haze near horizon
    float t = max(dir.y, 0.0);
    float skyGradient = pow(t, 0.6); // Stronger curve for more haze at horizon
    
    // Base sky color
    vec3 skyColor = mix(horizonColor, zenithColor, skyGradient);
    
    // Calculate sun effect
    float sunDot = dot(dir, lightDir);
    
    // Sun disk - sharp falloff for defined sun
    float sunDisk = smoothstep(0.9995, 0.9998, sunDot);
    
    // Sun halo - larger, softer glow around sun
    float sunHalo = pow(max(sunDot, 0.0), 32.0) * 0.3;
    
    // Sun corona - medium glow
    float sunCorona = pow(max(sunDot, 0.0), 8.0) * 0.15;
    
    // Only show sun if it's above horizon
    float sunVisibility = step(0.0, lightDir.y);
    
    // Combine sun effects
    vec3 sunContribution = (sunDisk * sunColor + 
                           sunHalo * sunHaloColor + 
                           sunCorona * sunHaloColor) * sunVisibility;
    
    // Add atmospheric scattering near sun (reddish tint when sun is low)
    float sunHeight = lightDir.y;
    float scatteringAmount = (1.0 - sunHeight) * 0.2;
    vec3 scatteringColor = vec3(1.0, 0.7, 0.5);
    float scatteringFactor = pow(max(sunDot, 0.0), 4.0) * scatteringAmount;
    
    // Combine all effects
    vec3 finalColor = skyColor + sunContribution + (scatteringColor * scatteringFactor);
    
    return vec4(finalColor, 1.0);
}

void main() {
    outColor = getSkyColor(inTexCoord);
}
