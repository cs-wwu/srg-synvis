// core/textures/manager.go
package textures

// TODO: This file is very much so untested.

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"log"
	"os"
	"sort"
	"sync"
	"time"
	"unsafe"

	"github.com/vulkan-go/asche"

	vk "github.com/vulkan-go/vulkan"
)

// TextureManager manages GPU texture resources with efficient loading and caching
type TextureManager struct {
	device      vk.Device
	ctx         asche.Context
	memProps    vk.PhysicalDeviceMemoryProperties
	commandPool vk.CommandPool
	queue       vk.Queue

	textures    map[string]*Texture
	maxTextures int

	mu sync.RWMutex
}

// Texture represents a loaded GPU texture resource
type Texture struct {
	ID       string
	Image    vk.Image
	Memory   vk.DeviceMemory
	View     vk.ImageView
	Sampler  vk.Sampler
	Width    uint32
	Height   uint32
	Format   vk.Format
	Layout   vk.ImageLayout
	InUse    bool
	LastUsed int64
}

// NewTextureManager creates a new texture manager
func NewTextureManager(device vk.Device, ctx asche.Context, maxTextures int) *TextureManager {
	// Get memory properties needed for allocations
	memProps := ctx.Platform().MemoryProperties()

	// Create a command pool for texture operations
	var commandPool vk.CommandPool
	ret := vk.CreateCommandPool(device, &vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: ctx.Platform().GraphicsQueueFamilyIndex(),
		Flags:            vk.CommandPoolCreateFlags(vk.CommandPoolCreateResetCommandBufferBit),
	}, nil, &commandPool)

	if ret != vk.Success {
		log.Printf("Failed to create command pool for texture manager: %d", ret)
	}

	return &TextureManager{
		device:      device,
		ctx:         ctx,
		memProps:    memProps,
		commandPool: commandPool,
		queue:       ctx.Platform().GraphicsQueue(),
		textures:    make(map[string]*Texture),
		maxTextures: maxTextures,
	}
}

// LoadTexture loads a texture from a file path
func (tm *TextureManager) LoadTexture(id string, filepath string) (*Texture, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if already loaded
	if texture, exists := tm.textures[id]; exists {
		texture.InUse = true
		texture.LastUsed = time.Now().UnixNano()
		return texture, nil
	}

	// Check if we need to evict a texture
	if len(tm.textures) >= tm.maxTextures {
		tm.evictLeastRecentlyUsed()
	}

	// Load image data
	imgData, width, height, err := loadImageData(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to load image data: %w", err)
	}

	// Create staging buffer
	stagingBuffer, stagingMemory := tm.createStagingBuffer(imgData)
	defer func() {
		vk.DestroyBuffer(tm.device, stagingBuffer, nil)
		vk.FreeMemory(tm.device, stagingMemory, nil)
	}()

	// Create image
	format := vk.FormatR8g8b8a8Unorm
	image, imageMemory, err := tm.createImage(uint32(width), uint32(height), format)
	if err != nil {
		return nil, fmt.Errorf("failed to create image: %w", err)
	}

	// Transition image layout for copy
	tm.transitionImageLayout(image, vk.ImageLayoutUndefined, vk.ImageLayoutTransferDstOptimal)

	// Copy data to image
	tm.copyBufferToImage(stagingBuffer, image, uint32(width), uint32(height))

	// Transition image layout for shader access
	tm.transitionImageLayout(image, vk.ImageLayoutTransferDstOptimal, vk.ImageLayoutShaderReadOnlyOptimal)

	// Create image view
	view, err := tm.createImageView(image, format)
	if err != nil {
		vk.DestroyImage(tm.device, image, nil)
		vk.FreeMemory(tm.device, imageMemory, nil)
		return nil, fmt.Errorf("failed to create image view: %w", err)
	}

	// Create sampler
	sampler, err := tm.createSampler()
	if err != nil {
		vk.DestroyImageView(tm.device, view, nil)
		vk.DestroyImage(tm.device, image, nil)
		vk.FreeMemory(tm.device, imageMemory, nil)
		return nil, fmt.Errorf("failed to create sampler: %w", err)
	}

	// Create and store the texture
	texture := &Texture{
		ID:       id,
		Image:    image,
		Memory:   imageMemory,
		View:     view,
		Sampler:  sampler,
		Width:    uint32(width),
		Height:   uint32(height),
		Format:   format,
		Layout:   vk.ImageLayoutShaderReadOnlyOptimal,
		InUse:    true,
		LastUsed: time.Now().UnixNano(),
	}

	tm.textures[id] = texture
	return texture, nil
}

// ReleaseTexture marks a texture as no longer in use
func (tm *TextureManager) ReleaseTexture(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if texture, exists := tm.textures[id]; exists {
		texture.InUse = false
		texture.LastUsed = time.Now().UnixNano()
	}
}

// GetTexture gets a texture by ID if it exists
func (tm *TextureManager) GetTexture(id string) *Texture {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	texture, exists := tm.textures[id]
	if !exists {
		return nil
	}
	// Only update LastUsed if the texture is actually in use
	if texture.InUse {
		texture.LastUsed = time.Now().UnixNano()
	}
	return texture
}

// DestroyTexture removes a texture and frees its resources
func (tm *TextureManager) DestroyTexture(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if texture, exists := tm.textures[id]; exists {
		tm.destroyTextureResources(texture)
		delete(tm.textures, id)
	}
}

// DestroyAll destroys all textures and frees the manager's resources
func (tm *TextureManager) DestroyAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, texture := range tm.textures {
		tm.destroyTextureResources(texture)
	}

	tm.textures = make(map[string]*Texture)
	if tm.commandPool != vk.NullCommandPool {
		vk.DestroyCommandPool(tm.device, tm.commandPool, nil)
		tm.commandPool = vk.NullCommandPool
	}
}

// GetStats returns statistics about the texture manager
func (tm *TextureManager) GetStats() map[string]any {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := map[string]any{
		"TotalTextures": len(tm.textures),
		"MaxTextures":   tm.maxTextures,
		"TexturesInUse": 0,
		"TotalMemory":   uint64(0),
	}

	inUseCount := 0
	totalMemory := uint64(0)

	for _, texture := range tm.textures {
		if texture.InUse {
			inUseCount++
		}
		// Approximate memory usage (RGBA8 = 4 bytes per pixel)
		totalMemory += uint64(texture.Width * texture.Height * 4)
	}

	stats["TexturesInUse"] = inUseCount
	stats["TotalMemory"] = totalMemory

	return stats
}

// evictLeastRecentlyUsed removes the least recently used texture
func (tm *TextureManager) evictLeastRecentlyUsed() {
	// Find candidates that are not in use
	candidates := make([]*Texture, 0)
	for _, texture := range tm.textures {
		if !texture.InUse {
			candidates = append(candidates, texture)
		}
	}

	if len(candidates) == 0 {
		log.Println("Warning: No textures available for eviction, all are in use")
		return
	}

	// Sort by last used time (oldest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LastUsed < candidates[j].LastUsed
	})

	// Evict the oldest
	toEvict := candidates[0]
	tm.destroyTextureResources(toEvict)
	delete(tm.textures, toEvict.ID)

	log.Printf("Evicted texture: %s", toEvict.ID)
}

// destroyTextureResources cleans up a texture's Vulkan resources
func (tm *TextureManager) destroyTextureResources(texture *Texture) {
	if texture.Sampler != vk.NullSampler {
		vk.DestroySampler(tm.device, texture.Sampler, nil)
		texture.Sampler = vk.NullSampler
	}
	if texture.View != vk.NullImageView {
		vk.DestroyImageView(tm.device, texture.View, nil)
		texture.View = vk.NullImageView
	}
	if texture.Image != vk.NullImage {
		vk.DestroyImage(tm.device, texture.Image, nil)
		texture.Image = vk.NullImage
	}
	if texture.Memory != vk.NullDeviceMemory {
		vk.FreeMemory(tm.device, texture.Memory, nil)
		texture.Memory = vk.NullDeviceMemory
	}
}

// Helper methods for creating and managing Vulkan resources

// loadImageData loads image data from a file
func loadImageData(filepath string) ([]byte, int, int, error) {
	// Read the file
	file, err := os.Open(filepath)
	if err != nil {
		return nil, 0, 0, err
	}
	defer file.Close()

	// Decode the image
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, 0, 0, err
	}

	// Convert to RGBA
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	return rgba.Pix, width, height, nil
}

// createStagingBuffer creates a staging buffer for uploading texture data
func (tm *TextureManager) createStagingBuffer(data []byte) (vk.Buffer, vk.DeviceMemory) {
	// Create buffer
	var buffer vk.Buffer
	ret := vk.CreateBuffer(tm.device, &vk.BufferCreateInfo{
		SType: vk.StructureTypeBufferCreateInfo,
		Size:  vk.DeviceSize(len(data)),
		Usage: vk.BufferUsageFlags(vk.BufferUsageTransferSrcBit),
	}, nil, &buffer)

	if ret != vk.Success {
		log.Fatalf("Failed to create staging buffer: %d", ret)
	}

	// Get memory requirements
	var memReqs vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(tm.device, buffer, &memReqs)
	memReqs.Deref()

	// Find suitable memory type
	memoryTypeIndex := tm.findMemoryType(
		memReqs.MemoryTypeBits,
		vk.MemoryPropertyHostVisibleBit|vk.MemoryPropertyHostCoherentBit,
	)

	// Allocate memory
	var memory vk.DeviceMemory
	ret = vk.AllocateMemory(tm.device, &vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: memoryTypeIndex,
	}, nil, &memory)

	if ret != vk.Success {
		vk.DestroyBuffer(tm.device, buffer, nil)
		log.Fatalf("Failed to allocate staging buffer memory: %d", ret)
	}

	// Bind memory to buffer
	ret = vk.BindBufferMemory(tm.device, buffer, memory, 0)
	if ret != vk.Success {
		vk.FreeMemory(tm.device, memory, nil)
		vk.DestroyBuffer(tm.device, buffer, nil)
		log.Fatalf("Failed to bind staging buffer memory: %d", ret)
	}

	// Copy data to buffer
	var pData unsafe.Pointer
	ret = vk.MapMemory(tm.device, memory, 0, vk.DeviceSize(len(data)), 0, &pData)
	if ret != vk.Success {
		log.Fatalf("Failed to map memory: %d", ret)
	}

	vk.Memcopy(pData, data)
	vk.UnmapMemory(tm.device, memory)

	return buffer, memory
}

// createImage creates a Vulkan image for the texture
func (tm *TextureManager) createImage(width, height uint32, format vk.Format) (vk.Image, vk.DeviceMemory, error) {
	// Create image
	var image vk.Image
	ret := vk.CreateImage(tm.device, &vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d,
		Format:    format,
		Extent: vk.Extent3D{
			Width:  width,
			Height: height,
			Depth:  1,
		},
		MipLevels:     1,
		ArrayLayers:   1,
		Samples:       vk.SampleCount1Bit,
		Tiling:        vk.ImageTilingOptimal,
		Usage:         vk.ImageUsageFlags(vk.ImageUsageTransferDstBit) | vk.ImageUsageFlags(vk.ImageUsageSampledBit),
		SharingMode:   vk.SharingModeExclusive,
		InitialLayout: vk.ImageLayoutUndefined,
	}, nil, &image)

	if ret != vk.Success {
		return vk.NullImage, vk.NullDeviceMemory, fmt.Errorf("failed to create image: %d", ret)
	}

	// Get memory requirements
	var memReqs vk.MemoryRequirements
	vk.GetImageMemoryRequirements(tm.device, image, &memReqs)
	memReqs.Deref()

	// Find suitable memory type
	memoryTypeIndex := tm.findMemoryType(
		memReqs.MemoryTypeBits,
		vk.MemoryPropertyDeviceLocalBit,
	)

	// Allocate memory
	var memory vk.DeviceMemory
	ret = vk.AllocateMemory(tm.device, &vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: memoryTypeIndex,
	}, nil, &memory)

	if ret != vk.Success {
		vk.DestroyImage(tm.device, image, nil)
		return vk.NullImage, vk.NullDeviceMemory, fmt.Errorf("failed to allocate image memory: %d", ret)
	}

	// Bind memory to image
	ret = vk.BindImageMemory(tm.device, image, memory, 0)
	if ret != vk.Success {
		vk.DestroyImage(tm.device, image, nil)
		vk.FreeMemory(tm.device, memory, nil)
		return vk.NullImage, vk.NullDeviceMemory, fmt.Errorf("failed to bind image memory: %d", ret)
	}

	return image, memory, nil
}

// createImageView creates an image view for the texture
func (tm *TextureManager) createImageView(image vk.Image, format vk.Format) (vk.ImageView, error) {
	var view vk.ImageView
	ret := vk.CreateImageView(tm.device, &vk.ImageViewCreateInfo{
		SType:    vk.StructureTypeImageViewCreateInfo,
		Image:    image,
		ViewType: vk.ImageViewType2d,
		Format:   format,
		Components: vk.ComponentMapping{
			R: vk.ComponentSwizzleIdentity,
			G: vk.ComponentSwizzleIdentity,
			B: vk.ComponentSwizzleIdentity,
			A: vk.ComponentSwizzleIdentity,
		},
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
			BaseMipLevel:   0,
			LevelCount:     1,
			BaseArrayLayer: 0,
			LayerCount:     1,
		},
	}, nil, &view)

	if ret != vk.Success {
		return vk.NullImageView, fmt.Errorf("failed to create image view: %d", ret)
	}

	return view, nil
}

// createSampler creates a sampler for the texture
func (tm *TextureManager) createSampler() (vk.Sampler, error) {
	var sampler vk.Sampler
	ret := vk.CreateSampler(tm.device, &vk.SamplerCreateInfo{
		SType:                   vk.StructureTypeSamplerCreateInfo,
		MagFilter:               vk.FilterLinear,
		MinFilter:               vk.FilterLinear,
		MipmapMode:              vk.SamplerMipmapModeLinear,
		AddressModeU:            vk.SamplerAddressModeRepeat,
		AddressModeV:            vk.SamplerAddressModeRepeat,
		AddressModeW:            vk.SamplerAddressModeRepeat,
		MipLodBias:              0.0,
		AnisotropyEnable:        vk.False,
		MaxAnisotropy:           1.0,
		CompareEnable:           vk.False,
		CompareOp:               vk.CompareOpAlways,
		MinLod:                  0.0,
		MaxLod:                  0.0,
		BorderColor:             vk.BorderColorIntOpaqueBlack,
		UnnormalizedCoordinates: vk.False,
	}, nil, &sampler)

	if ret != vk.Success {
		return vk.NullSampler, fmt.Errorf("failed to create sampler: %d", ret)
	}

	return sampler, nil
}

// transitionImageLayout transitions an image's layout
func (tm *TextureManager) transitionImageLayout(image vk.Image, oldLayout, newLayout vk.ImageLayout) {
	// Create command buffer
	commandBuffer := tm.beginSingleTimeCommands()

	// Create barrier
	barrier := vk.ImageMemoryBarrier{
		SType:               vk.StructureTypeImageMemoryBarrier,
		OldLayout:           oldLayout,
		NewLayout:           newLayout,
		SrcQueueFamilyIndex: vk.QueueFamilyIgnored,
		DstQueueFamilyIndex: vk.QueueFamilyIgnored,
		Image:               image,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
			BaseMipLevel:   0,
			LevelCount:     1,
			BaseArrayLayer: 0,
			LayerCount:     1,
		},
	}

	// Set source and destination access masks based on layouts
	var srcStage, dstStage vk.PipelineStageFlagBits

	if oldLayout == vk.ImageLayoutUndefined && newLayout == vk.ImageLayoutTransferDstOptimal {
		barrier.SrcAccessMask = vk.AccessFlags(0)
		barrier.DstAccessMask = vk.AccessFlags(vk.AccessTransferWriteBit)

		srcStage = vk.PipelineStageTopOfPipeBit
		dstStage = vk.PipelineStageTransferBit
	} else if oldLayout == vk.ImageLayoutTransferDstOptimal && newLayout == vk.ImageLayoutShaderReadOnlyOptimal {
		barrier.SrcAccessMask = vk.AccessFlags(vk.AccessTransferWriteBit)
		barrier.DstAccessMask = vk.AccessFlags(vk.AccessShaderReadBit)

		srcStage = vk.PipelineStageTransferBit
		dstStage = vk.PipelineStageFragmentShaderBit
	} else {
		log.Fatalf("Unsupported layout transition: %d -> %d", oldLayout, newLayout)
	}

	// Pipeline barrier
	vk.CmdPipelineBarrier(
		commandBuffer,
		vk.PipelineStageFlags(srcStage),
		vk.PipelineStageFlags(dstStage),
		0,
		0, nil,
		0, nil,
		1, []vk.ImageMemoryBarrier{barrier},
	)

	// Submit command buffer
	tm.endSingleTimeCommands(commandBuffer)
}

// copyBufferToImage copies data from a buffer to an image
func (tm *TextureManager) copyBufferToImage(buffer vk.Buffer, image vk.Image, width, height uint32) {
	// Create command buffer
	commandBuffer := tm.beginSingleTimeCommands()

	// Set up copy region
	region := vk.BufferImageCopy{
		BufferOffset:      0,
		BufferRowLength:   0, // Tightly packed
		BufferImageHeight: 0, // Tightly packed
		ImageSubresource: vk.ImageSubresourceLayers{
			AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
			MipLevel:       0,
			BaseArrayLayer: 0,
			LayerCount:     1,
		},
		ImageOffset: vk.Offset3D{X: 0, Y: 0, Z: 0},
		ImageExtent: vk.Extent3D{
			Width:  width,
			Height: height,
			Depth:  1,
		},
	}

	// Copy buffer to image
	vk.CmdCopyBufferToImage(
		commandBuffer,
		buffer,
		image,
		vk.ImageLayoutTransferDstOptimal,
		1,
		[]vk.BufferImageCopy{region},
	)

	// Submit command buffer
	tm.endSingleTimeCommands(commandBuffer)
}

// beginSingleTimeCommands begins a one-time command buffer
func (tm *TextureManager) beginSingleTimeCommands() vk.CommandBuffer {
	// Allocate command buffer
	var commandBuffer vk.CommandBuffer
	ret := vk.AllocateCommandBuffers(tm.device, &vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        tm.commandPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: 1,
	}, []vk.CommandBuffer{commandBuffer})

	if ret != vk.Success {
		log.Fatalf("Failed to allocate command buffer: %d", ret)
	}

	// Begin command buffer
	ret = vk.BeginCommandBuffer(commandBuffer, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageOneTimeSubmitBit),
	})

	if ret != vk.Success {
		log.Fatalf("Failed to begin command buffer: %d", ret)
	}

	return commandBuffer
}

// endSingleTimeCommands ends and submits a one-time command buffer
func (tm *TextureManager) endSingleTimeCommands(commandBuffer vk.CommandBuffer) {
	// End command buffer
	ret := vk.EndCommandBuffer(commandBuffer)
	if ret != vk.Success {
		log.Fatalf("Failed to end command buffer: %d", ret)
	}

	// Submit command buffer
	ret = vk.QueueSubmit(tm.queue, 1, []vk.SubmitInfo{{
		SType:              vk.StructureTypeSubmitInfo,
		CommandBufferCount: 1,
		PCommandBuffers:    []vk.CommandBuffer{commandBuffer},
	}}, vk.NullFence)

	if ret != vk.Success {
		log.Fatalf("Failed to submit command buffer: %d", ret)
	}

	// Wait for queue to finish
	ret = vk.QueueWaitIdle(tm.queue)
	if ret != vk.Success {
		log.Fatalf("Failed to wait for queue: %d", ret)
	}

	// Free command buffer
	vk.FreeCommandBuffers(tm.device, tm.commandPool, 1, []vk.CommandBuffer{commandBuffer})
}

// findMemoryType finds a suitable memory type
func (tm *TextureManager) findMemoryType(typeFilter uint32, properties vk.MemoryPropertyFlagBits) uint32 {
	for i := uint32(0); i < tm.memProps.MemoryTypeCount; i++ {
		if (typeFilter&(1<<i)) != 0 && (tm.memProps.MemoryTypes[i].PropertyFlags&vk.MemoryPropertyFlags(properties)) == vk.MemoryPropertyFlags(properties) {
			return i
		}
	}

	log.Fatalf("Failed to find suitable memory type")
	return 0
}
