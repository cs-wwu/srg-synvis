#version 450
#extension GL_ARB_separate_shader_objects : enable
#extension GL_ARB_shading_language_420pack : enable

layout(location = 0) in vec3 inPosition;
layout(location = 1) in vec2 inTexCoord;

layout (location = 0) out vec4 texcoord;

out gl_PerVertex {
        vec4 gl_Position;
};

void main() 
{
   texcoord = inTexCoord;
   gl_Position = inPosition;
}