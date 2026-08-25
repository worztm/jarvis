package brain

// SITES: canonical name -> URL. Every entry is openable by fuzzy voice match.
var Sites = map[string]string{
	// search / knowledge
	"google": "https://www.google.com", "bing": "https://www.bing.com",
	"duckduckgo": "https://duckduckgo.com", "wikipedia": "https://www.wikipedia.org",
	"britannica": "https://www.britannica.com", "quora": "https://www.quora.com",
	"medium": "https://medium.com", "reddit": "https://www.reddit.com",
	// ai
	"chatgpt": "https://chatgpt.com", "claude": "https://claude.ai",
	"gemini": "https://gemini.google.com", "perplexity": "https://www.perplexity.ai",
	"copilot": "https://copilot.microsoft.com", "poe": "https://poe.com",
	"hugging face": "https://huggingface.co", "kaggle": "https://www.kaggle.com",
	// communication
	"gmail": "https://mail.google.com", "outlook mail": "https://outlook.live.com",
	"proton mail": "https://mail.proton.me", "yahoo mail": "https://mail.yahoo.com",
	"whatsapp web": "https://web.whatsapp.com", "telegram web": "https://web.telegram.org",
	"discord": "https://discord.com/app", "slack": "https://app.slack.com",
	"zoom": "https://zoom.us", "google meet": "https://meet.google.com",
	"microsoft teams": "https://teams.microsoft.com",
	// social
	"youtube": "https://www.youtube.com", "twitter": "https://x.com",
	"x": "https://x.com", "instagram": "https://www.instagram.com",
	"facebook": "https://www.facebook.com", "snapchat": "https://web.snapchat.com",
	"tiktok": "https://www.tiktok.com", "linkedin": "https://www.linkedin.com",
	"pinterest": "https://www.pinterest.com", "tumblr": "https://www.tumblr.com",
	"threads": "https://threads.net",
	// video / streaming
	"netflix": "https://www.netflix.com", "prime video": "https://primevideo.com",
	"hulu": "https://www.hulu.com", "disney plus": "https://www.disneyplus.com",
	"max": "https://www.max.com", "apple tv": "https://tv.apple.com",
	"crunchyroll": "https://www.crunchyroll.com", "twitch": "https://www.twitch.tv",
	"kick": "https://kick.com", "vimeo": "https://vimeo.com",
	"dailymotion": "https://www.dailymotion.com", "imdb": "https://www.imdb.com",
	"rotten tomatoes": "https://www.rottentomatoes.com", "letterboxd": "https://letterboxd.com",
	// music
	"spotify": "https://open.spotify.com", "apple music": "https://music.apple.com",
	"soundcloud": "https://soundcloud.com", "deezer": "https://www.deezer.com",
	"tidal": "https://tidal.com", "audiomack": "https://audiomack.com",
	"boomplay": "https://www.boomplay.com", "audible": "https://www.audible.com",
	"goodreads": "https://www.goodreads.com", "wattpad": "https://www.wattpad.com",
	"webtoon": "https://www.webtoons.com",
	// shopping
	"amazon": "https://www.amazon.com", "ebay": "https://www.ebay.com",
	"etsy": "https://www.etsy.com", "aliexpress": "https://www.aliexpress.com",
	"alibaba": "https://www.alibaba.com", "shein": "https://www.shein.com",
	"temu": "https://www.temu.com", "jumia": "https://www.jumia.com.ng",
	"konga": "https://www.konga.com", "jiji": "https://www.jiji.ng",
	// finance
	"paypal": "https://www.paypal.com", "payoneer": "https://www.payoneer.com",
	"wise": "https://wise.com", "stripe": "https://stripe.com",
	"binance": "https://www.binance.com", "coinbase": "https://www.coinbase.com",
	"trading view": "https://www.tradingview.com",
	// travel / maps
	"maps": "https://maps.google.com", "google maps": "https://maps.google.com",
	"uber": "https://m.uber.com", "bolt": "https://bolt.eu",
	"booking": "https://www.booking.com", "airbnb": "https://www.airbnb.com",
	"expedia": "https://www.expedia.com", "skyscanner": "https://www.skyscanner.net",
	"trip advisor": "https://www.tripadvisor.com",
	// dev
	"github": "https://github.com", "gitlab": "https://gitlab.com",
	"bitbucket": "https://bitbucket.org", "stack overflow": "https://stackoverflow.com",
	"npm": "https://www.npmjs.com", "pypi": "https://pypi.org",
	"docker hub": "https://hub.docker.com", "mdn": "https://developer.mozilla.org",
	"w3schools": "https://www.w3schools.com", "geeksforgeeks": "https://www.geeksforgeeks.org",
	"leetcode": "https://leetcode.com", "hackerrank": "https://www.hackerrank.com",
	"codeforces": "https://codeforces.com", "codewars": "https://www.codewars.com",
	"codecademy": "https://www.codecademy.com", "free code camp": "https://www.freecodecamp.org",
	"codepen": "https://codepen.io", "codesandbox": "https://codesandbox.io",
	"replit": "https://replit.com", "stack blitz": "https://stackblitz.com",
	"js fiddle": "https://jsfiddle.net", "vercel": "https://vercel.com",
	"netlify": "https://www.netlify.com", "cloud flare": "https://dash.cloudflare.com",
	"overleaf": "https://www.overleaf.com", "pastebin": "https://pastebin.com",
	// learning
	"coursera": "https://www.coursera.org", "udemy": "https://www.udemy.com",
	"edx": "https://www.edx.org", "khan academy": "https://www.khanacademy.org",
	"duolingo": "https://www.duolingo.com", "quizlet": "https://quizlet.com",
	"brainly": "https://brainly.com", "chegg": "https://www.chegg.com",
	"google classroom": "https://classroom.google.com",
	"desmos": "https://www.desmos.com/calculator", "symbolab": "https://www.symbolab.com",
	"wolfram alpha": "https://www.wolframalpha.com", "photo math": "https://photomath.com",
	// design
	"figma": "https://www.figma.com", "canva": "https://www.canva.com",
	"photo pea": "https://www.photopea.com", "unsplash": "https://unsplash.com",
	"pexels": "https://www.pexels.com", "pixabay": "https://pixabay.com",
	"giphy": "https://giphy.com", "imgur": "https://imgur.com",
	"behance": "https://www.behance.net", "dribbble": "https://dribbble.com",
	// productivity
	"notion": "https://www.notion.so", "trello": "https://trello.com",
	"asana": "https://app.asana.com", "airtable": "https://airtable.com",
	"click up": "https://clickup.com", "evernote": "https://www.evernote.com",
	// news / sports
	"bbc": "https://www.bbc.com", "cnn": "https://www.cnn.com",
	"al jazeera": "https://www.aljazeera.com", "the guardian": "https://www.theguardian.com",
	"new york times": "https://www.nytimes.com", "forbes": "https://www.forbes.com",
	"bloomberg": "https://www.bloomberg.com", "tech crunch": "https://techcrunch.com",
	"the verge": "https://www.theverge.com", "wired": "https://www.wired.com",
	"espn": "https://www.espn.com", "flash score": "https://www.flashscore.com",
	"sofa score": "https://www.sofascore.com", "goal": "https://www.goal.com",
	"nba": "https://www.nba.com",
	// utilities
	"translate": "https://translate.google.com", "drive": "https://drive.google.com",
	"google docs": "https://docs.google.com/document/u/0/",
	"google sheets": "https://docs.google.com/spreadsheets/u/0/",
	"google slides": "https://docs.google.com/presentation/u/0/",
	"google photos": "https://photos.google.com", "dropbox": "https://www.dropbox.com",
	"weather channel": "https://weather.com",
}

// Aliases: spoken forms that map straight to a canonical target.
var Aliases = map[string]string{
	"you tube": "youtube", "you tab": "youtube", "utube": "youtube",
	"u tube": "youtube", "view tube": "youtube", "face book": "facebook",
	"linked in": "linkedin", "git hub": "github", "git lab": "gitlab",
	"teach crash": "tech crunch", "net flicks": "netflix", "whats app": "whatsapp",
	"what's app": "whatsapp", "g mail": "gmail", "grams": "gmail",
	"brave browser": "brave", "telegram desktop": "telegram", "telegram app": "telegram",
	"doc": "google docs", "sheets": "google sheets", "i am db": "imdb",
	"wolfram": "wolfram alpha", "coin base": "coinbase", "ali express": "aliexpress",
	"khan academy": "khan academy", "free code camp": "free code camp",
}

// Apps: canonical name -> launch candidates tried in order. Names ending in
// ':' are URI/protocol launches; bare names go through PATH lookup; absolute
// paths are used directly.
var Apps = map[string][]string{
	"notepad":            {"notepad"},
	"calculator":         {"calc"},
	"paint":              {"mspaint"},
	"file explorer":      {"explorer"},
	"files":              {"explorer"},
	"settings":           {"ms-settings:"},
	"task manager":       {"taskmgr"},
	"command prompt":     {"cmd"},
	"powershell":         {"powershell"},
	"terminal":           {"wt"},
	"word":               {"winword"},
	"excel":              {"excel"},
	"powerpoint":         {"powerpnt"},
	"outlook":            {"outlook"},
	"vs code":            {"code"},
	"visual studio code": {"code"},
	"spotify desktop":    {"spotify:"},
	// native windows apps (paths + start menu resolution)
	"whatsapp":           {"whatsapp:"},
	"telegram":           {`%APPDATA%\Telegram Desktop\Telegram.exe`},
	"brave":              {`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`, `C:\Program Files (x86)\BraveSoftware\Brave-Browser\Application\brave.exe`},
	"chrome":             {`C:\Program Files\Google\Chrome\Application\chrome.exe`, `C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`, "chrome"},
	"edge":               {`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, `C:\Program Files\Microsoft\Edge\Application\msedge.exe`, "msedge"},
	// windows utilities (resolved via cmd start / App Paths registry)
	"control panel":      {"control"},
	"registry editor":    {"regedit"},
	"device manager":     {"devmgmt.msc"},
	"disk management":    {"diskmgmt.msc"},
	"services":           {"services.msc"},
	"task scheduler":     {"taskschd.msc"},
	"event viewer":       {"eventvwr.msc"},
	"snipping tool":      {"snippingtool"},
	"screenshot tool":    {"snippingtool"},
	"wordpad":            {"write"},
	"magnifier":          {"magnify"},
	"on-screen keyboard": {"osk"},
	"volume mixer":       {"sndvol"},
	"character map":      {"charmap"},
	"camera":             {"microsoft.windows.camera:"},
	"microsoft store":    {"ms-windows-store:"},
	"store":              {"ms-windows-store:"},
	"xbox":               {"xbox:"},
	"bluetooth settings": {"ms-settings:bluetooth"},
	"wifi settings":      {"ms-settings:network-wifi"},
	"display settings":   {"ms-settings:display"},
	"sound settings":     {"ms-settings:sound"},
	"apps settings":      {"ms-settings:appsfeatures"},
	"windows update":     {"ms-settings:windowsupdate"},
}

// TargetCount is the total number of distinct openable things.
func TargetCount() int {
	return len(Sites) + len(Apps)
}
