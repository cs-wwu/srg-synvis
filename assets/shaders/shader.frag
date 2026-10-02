#version 450
#extension GL_ARB_separate_shader_objects : enable
#extension GL_ARB_shading_language_420pack : enable

layout(binding = 1) uniform sampler2D texSampler;

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
} ubo;

layout(location = 0) in vec3 fragColor;
layout(location = 1) in vec3 fragTexCoord;
layout(location = 2) in vec3 worldPos;
layout(location = 3) in float fogFactor;
layout(location = 4) in vec3 fragNormal;

layout(location = 0) out vec4 outColor;

// Better hash function for more randomness
float hash21(vec2 p) {
    vec3 p3 = fract(vec3(p.xyx) * vec3(0.1031, 0.1031, 0.1030));
    p3 += dot(p3, p3.yzx + 19.19);
    return fract((p3.x + p3.y) * p3.z);
}

// Gradient noise with better mixing
float gnoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    
    // Cubic Hermite curve for smoother interpolation
    vec2 u = f * f * (3.0 - 2.0 * f);
    
    // Four corners
    float a = hash21(i);
    float b = hash21(i + vec2(1.0, 0.0));
    float c = hash21(i + vec2(0.0, 1.0));
    float d = hash21(i + vec2(1.0, 1.0));
    
    // Mixing
    float x1 = mix(a, b, u.x);
    float x2 = mix(c, d, u.x);
    
    return mix(x1, x2, u.y);
}

// Fractional Brownian Motion with multiple octaves - for terrain
float fbm(vec2 p, int octaves) {
    float value = 0.0;
    float amplitude = 0.5;
    float frequency = 1.0;
    // Rotate coordinates for each octave to reduce directional artifacts
    mat2 rot = mat2(cos(0.5), sin(0.5), -sin(0.5), cos(0.5));
    
    for (int i = 0; i < octaves; i++) {
        value += amplitude * gnoise(p * frequency);
        // Rotate coordinates to break up patterns
        p = rot * p;
        frequency *= 2.17; // Not a power of 2 to reduce patterns
        amplitude *= 0.5;
    }
    
    return value;
}

// SIMPLIFIED water wave function with gentler movements
float waveHeight(vec2 position, float time) {
    // Reduced number of waves with lower frequencies for smoother motion
    float wave1 = sin(position.x * 0.023 + position.y * 0.031 + time * 0.4) * 0.03;
    float wave2 = sin(position.x * 0.02 - position.y * 0.027 + time * 0.3) * 0.02;
    
    // Very gentle third wave component for just a bit more complexity
    float wave3 = sin(position.x * 0.013 + position.y * 0.019 + time * 0.2) * 0.01;
    
    // Minimal noise - just enough to break perfect uniformity
    float noiseVal = gnoise(position * 0.03 + time * 0.05) * 0.01;
    
    return wave1 + wave2 + wave3 + noiseVal;
}

// Simpler fresnel effect calculation
float fresnel(vec3 normal, vec3 viewDir, float bias, float scale, float power) {
    float NdotV = max(0.0, dot(normal, viewDir));
    return bias + scale * pow(1.0 - NdotV, power);
}

// Very simple terrain height warning function
// Only shows warning when terrain is at or near camera height
float getTerrainWarningOpacity(float terrainHeight, float cameraHeight) {
    // Height difference between terrain and camera
    float heightDiff = terrainHeight - cameraHeight;
    
    // Only show warning when terrain is at or above camera,
    // with a small transition zone (20 units) below camera height
    float transitionZone = -20.0;
    
    if (heightDiff >= 0.0) {
        // Terrain is at or above camera - full warning
        return 0.8;  // 80% opacity for warning
    }
    else if (heightDiff >= transitionZone) {
        // Transition zone - fade in warning as terrain gets closer
        return 0.8 * (1.0 - (heightDiff / transitionZone));
    }
    
    // Normal terrain - no warning
    return 0.0;
}

void main() {
    // Process terrain or water
    vec4 baseColor;
    vec3 normNormal = normalize(fragNormal);
    float steepness = abs(normNormal.y);
    
    if (fragTexCoord.z > 0.5) {
        // SIMPLIFIED WATER RENDERING
        float waterDepth = 1.0 - fragColor.r * 0.5;
        
        // Enhanced water color palette
        vec3 deepWater = vec3(0.0, 0.05, 0.15);
        vec3 midWater = vec3(0.0, 0.15, 0.35);
        vec3 shallowWater = vec3(0.1, 0.5, 0.8);
        
        // Smoother depth color transition
        vec3 waterColor;
        if (waterDepth < 0.3) {
            waterColor = mix(shallowWater, midWater, waterDepth / 0.3);
        } else {
            waterColor = mix(midWater, deepWater, (waterDepth - 0.3) / 0.7);
        }
        
        // Gentler wave calculation
        float waveSum = waveHeight(worldPos.xz, ubo.time);
        
        // Apply subtle color variation based on wave height
        waterColor = mix(waterColor, waterColor * 1.1, waveSum * 3.0 + 0.5);
        
        // Simpler lighting 
        vec3 lightDir = normalize(ubo.lightDir);
        vec3 viewDir = normalize(-worldPos);
        
        // Simplified wave normal - much gentler
        vec3 waveNormal = normalize(vec3(
            sin(worldPos.x * 0.02 + worldPos.z * 0.03 + ubo.time * 0.3) * 0.1,
            0.9, // Stronger Y component for more stable reflections
            sin(worldPos.z * 0.02 + worldPos.x * 0.01 + ubo.time * 0.2) * 0.1
        ));
        
        // Specular highlights with smoother normal
        vec3 reflectDir = reflect(-lightDir, waveNormal);
        float spec = pow(max(dot(viewDir, reflectDir), 0.0), 32.0);
        
        // Gentler fresnel effect
        float fresnelFactor = fresnel(waveNormal, viewDir, 0.05, 0.5, 3.0);
        
        // Sky reflection for water - use simple gradient
        vec3 skyReflection = mix(vec3(0.8, 0.9, 0.95), vec3(0.05, 0.15, 0.5), 
                                max(0.0, reflect(-viewDir, waveNormal).y));
        
        // Combine with reduced fresnel influence
        waterColor = mix(waterColor, skyReflection, fresnelFactor * 0.4);
        
        // Add soft specular highlight
        vec3 specular = spec * vec3(1.0, 1.0, 0.95) * 0.3;
        
        // Final water color
        baseColor = vec4(waterColor + specular, 0.9 + waveSum * 0.5);
    } else {
        // TERRAIN RENDERING - Using the improved noise functions
        vec4 texColor = texture(texSampler, fragTexCoord.xy);
        
        // Base green palette - vibrant but natural greens
        vec3 green1 = vec3(0.05, 0.40, 0.05); // Darker green
        vec3 green2 = vec3(0.10, 0.50, 0.10); // Medium green
        vec3 green3 = vec3(0.16, 0.55, 0.12); // Lighter green
        vec3 green4 = vec3(0.20, 0.60, 0.15); // Bright green
        vec3 green5 = vec3(0.13, 0.45, 0.08); // Another medium variation
        
        // Rock and snow colors
        vec3 rockColor = vec3(0.65, 0.63, 0.60); // Light gray rock
        vec3 snowColor = vec3(0.9, 0.9, 0.95);   // Snow
        
        // Use better noise functions for terrain
        float n1 = fbm(worldPos.xz * 0.01, 3);
        float n2 = fbm(worldPos.xz * 0.02 + vec2(42.5, 13.2), 3);
        float n3 = fbm(worldPos.xz * 0.04 + vec2(11.9, 78.6), 2);
        float n4 = fbm(worldPos.xz * 0.08 + vec2(87.2, 33.4), 2);
        
        // Create a random green color using weighted noise
        float greenSelector = n1 * 0.5 + n2 * 0.3 + n3 * 0.15 + n4 * 0.05;
        
        // Create a random blend between the green colors
        vec3 forestColor;
        if (greenSelector < 0.2) {
            forestColor = green1;
        } else if (greenSelector < 0.4) {
            forestColor = green5;
        } else if (greenSelector < 0.6) {
            forestColor = green2;
        } else if (greenSelector < 0.8) {
            forestColor = green3;
        } else {
            forestColor = green4;
        }
        
        // Add small-scale variation with better noise
        float detailVar = fbm(worldPos.xz * 0.3, 2) * 0.1 - 0.05;
        forestColor = forestColor * (1.0 + detailVar);
        
        // Calculate mix factor for rock/forest based on steepness
        float slopeFactor = smoothstep(0.0, 0.5, steepness); // Steeper threshold for rock
        vec3 terrainColor = mix(rockColor, forestColor, slopeFactor);
        
        // Add height-based coloring - snow on peaks
        float heightFactor = smoothstep(1500.0, 2000.0, worldPos.y);
        terrainColor = mix(terrainColor, snowColor, heightFactor);
        
        // Add slight variation to rock color based on direction (warmer on south facing)
        if (slopeFactor < 0.5) { // Only for rocky areas
            // Get the slope direction (which way is it facing)
            vec2 slopeDir = normalize(normNormal.xz);
            
            // Check if facing south (roughly +Z in your world space)
            float southness = slopeDir.y; // More positive = more south-facing
            
            // Add subtle warm tint to south-facing rocks, cool tint to north-facing
            terrainColor += vec3(southness * 0.05, southness * 0.03, -southness * 0.05) * (1.0 - slopeFactor * 2.0);
        }
        
        // Disable warnings is 1.0, run the code if disableWarnings is 0.0
        if (ubo.disableWarnings < 0.5) {
            // Apply warning coloring based on height relative to camera position
            float warningOpacity = getTerrainWarningOpacity(worldPos.y, ubo.cameraPos.y);
            
            // Add red warning overlay only when terrain is at or near camera height
            if (warningOpacity > 0.0) {
                vec3 warningColor = vec3(1.0, 0.0, 0.0); // Pure red warning
                terrainColor = mix(terrainColor, warningColor, warningOpacity);
            }
        }
        
        // Use texture if available
        if (texColor.a > 0.5) {
            baseColor = texColor * vec4(fragColor, 1.0); 
        } else {
            baseColor = vec4(terrainColor * fragColor, 1.0);
        }
    }
    
    // Use consistent horizon/fog color for blending
    // This should match the horizon color from the skybox
    vec3 fogColor = vec3(0.8, 0.9, 0.95); // Same as skybox horizonColor
    
    // Final blending with fog using pre-calculated fog factor
    // fogFactor: 1.0 = no fog (show baseColor), 0.0 = full fog (show fogColor)
    outColor = mix(vec4(fogColor, 1.0), baseColor, fogFactor);
}
