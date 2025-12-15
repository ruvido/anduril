# Actions to test the effectiveness of import call

1. take 3 test pictures
   1. one image contains full exif - eg file coming straight from a camera: full-exif/image.jpg
   2. one image comes from messaging apps - no exif, date embedded in filename: message/image.jpg
   3. one image has no exif: no-exif/image.jpg
2. import 1,2,3
3. import 3,1,2
4. import 2,3,1