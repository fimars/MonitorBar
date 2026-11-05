package display

// edidDimensions reads the preferred detailed timing and physical panel
// size. Scaled render buffers must never be treated as panel dimensions.
func edidDimensions(edid []byte) (width, height, widthMM, heightMM int) {
	if len(edid) < 128 {
		return
	}
	widthMM, heightMM = int(edid[21])*10, int(edid[22])*10
	if edid[54] == 0 && edid[55] == 0 {
		return
	}
	width = int(edid[56]) | int(edid[58]>>4)<<8
	height = int(edid[59]) | int(edid[61]>>4)<<8
	return
}
