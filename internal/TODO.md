# Interal TODOs

These are the internal system TODOs.

## TODO

### Cooper

Some of these changes will likely require a large amount of low-level changes
to the core engine. E.g., More than one map being loaded, not swapping entire
buffers when loading new chunks, etc. This may require new buffer management
strategies, or a better solution than the current pending/active buffer swap
solution (specifically for Chunk loading). I'm still not sure how to
implement the dynamic texture swapping.

- [ ] Fix background colors
  - [ ] Horizon fog
  - [ ] Blue sky
  - [ ] Better vertex colors? E.g.,
    - [ ] Gray on steep surface
    - [ ] Better randomized green colors
- [ ] Fix texture striping on some devices
  - [ ] Better road colorings? (Kyle/Katria)
- [ ] Dynamic texture loading, can swap in and out of buffers
- [ ] Better Dynamic Chunk Loading
  - [ ] Remove loading based on map
  - [ ] Load each image into memory
  - [ ] Only load chunks in a ring around the player
- [ ] Fix chunk boundaries (with stitching/skirts)
- [ ] Re-implement frustum culling based upon performance

### Longterm

- [ ] Refactor project
  - [ ] Separate inputs into separate files
  - [ ] More distinctly define "Debug" mode vs "Prod" mode
