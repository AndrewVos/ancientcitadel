-- up
UPDATE urls SET gfy_name = webmurl;
UPDATE urls SET gfy_name = replace(gfy_name, 'http://zippy.gfycat.com/', '');
UPDATE urls SET gfy_name = replace(gfy_name, 'http://giant.gfycat.com/', '');
UPDATE urls SET gfy_name = replace(gfy_name, 'http://fat.gfycat.com/', '');
UPDATE urls SET gfy_name = replace(gfy_name, '.webm', '');
