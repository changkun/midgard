// stamp writes a line under the title of the disk image's background, in
// the place and the look background.svg leaves room for: apple/build.sh
// writes the version there, for a person to say which one they have.
//
//	swift stamp.swift <text> <in.png> <out.png> [<in.png> <out.png>]...
//
// Each picture is background.svg drawn at some scale, which its width tells.
import AppKit

let args = CommandLine.arguments
guard args.count >= 4, args.count % 2 == 0 else {
    FileHandle.standardError.write("usage: stamp <text> <in.png> <out.png> [<in.png> <out.png>]...\n".data(using: .utf8)!)
    exit(2)
}
let text = args[1]

// as background.svg's line: centred at x 330, on y 314 from the top, 12.5pt
let (width, height) = (660.0, 400.0)
let (x, baseline, size) = (330.0, 314.0, 12.5)
let color = CGColor(srgbRed: 0x56 / 255.0, green: 0x64 / 255.0, blue: 0x8a / 255.0, alpha: 1)

for i in stride(from: 2, to: args.count, by: 2) {
    guard let image = NSImage(contentsOfFile: args[i]),
          let picture = image.cgImage(forProposedRect: nil, context: nil, hints: nil),
          let ctx = CGContext(data: nil, width: picture.width, height: picture.height, bitsPerComponent: 8,
                              bytesPerRow: 0, space: CGColorSpace(name: CGColorSpace.sRGB)!,
                              bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
    else {
        FileHandle.standardError.write("stamp: cannot read \(args[i])\n".data(using: .utf8)!)
        exit(1)
    }
    ctx.draw(picture, in: CGRect(x: 0, y: 0, width: picture.width, height: picture.height))
    // in the SVG's points, so that the type is 12.5pt at every scale
    let scale = Double(picture.width) / width
    ctx.scaleBy(x: scale, y: scale)
    let line = CTLineCreateWithAttributedString(NSAttributedString(string: text, attributes: [
        .font: NSFont.systemFont(ofSize: size),
        NSAttributedString.Key(kCTForegroundColorAttributeName as String): color,
    ]))
    let advance = CTLineGetTypographicBounds(line, nil, nil, nil)
    ctx.textPosition = CGPoint(x: x - advance / 2, y: height - baseline)
    CTLineDraw(line, ctx)

    guard let stamped = ctx.makeImage(),
          let png = NSBitmapImageRep(cgImage: stamped).representation(using: .png, properties: [:])
    else {
        FileHandle.standardError.write("stamp: cannot draw \(args[i + 1])\n".data(using: .utf8)!)
        exit(1)
    }
    do {
        try png.write(to: URL(fileURLWithPath: args[i + 1]))
    } catch {
        FileHandle.standardError.write("stamp: \(error.localizedDescription)\n".data(using: .utf8)!)
        exit(1)
    }
}
