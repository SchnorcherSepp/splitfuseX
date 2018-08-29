package fh

// Wieviel darf max. im fh Cache gespeichert werden (10 MB)
// Hinweis: Diese Cache-Größe ist pro FH und kann sich daher schnell summieren (RAM sparern!)
const MaxCacheSize = 10 * 1024 * 1024

// Wie weit soll VOR einem offset initial gelesen werden (1 MB)
// Im FileHandler nenne ich das pre-load.
const PreloadSize = 1 * 1024 * 1024

// Welche Dateigröße soll maximal gestreamt werden? (20 GB)
// Die Chunks können soweiso max. 1 GB groß werden.
const MaxFileSize = 20 * 1024 * 1024 * 1024

// Wie weit darf ungecached nach vorne gesprungen werden, bis ein Fehler kommt (50 MB)
// Vorspringen gedeutet, dass die Daten trotzdem bis zu dem Punkt geladen werden müssen.
const MaxForwardJump = 50 * 1024 * 1024

// Wieviel bytes können von google auf einmal empfangen werden (größe eines CacheElement).
// Das ist der read buffer beim Download.
const ReadBufferSize = 32768
