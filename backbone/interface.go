package backbone

import (
	"io"
	"time"
)

// Client ist ein Interface um auf Speicher wie Google Drive oder der lokalen Festplatte zuzugreifen.
type Client interface {
	// Read erlaubt den lesenden Zugriff auf eine Datei (identifiziert über die fileId).
	// Der offset erlaubt es, das Lesen an einer beliebigen Stelle zu beginnen (default 0)
	// Die fileSize muss nicht exakt die Dateigröße angeben. Dieser Wert sagt lediglich, wie weit max. gelesen wird.
	// Dabei kann der Stream bereits früher mit EOF enden! (default 20.000.000.000)
	// ACHTUNG: Am Ende .Close() nicht vergessen!
	Read(fileId string, offset int64, fileSize int64) (io.ReadCloser, error)

	// Trash verschiebt das angegebene Objekt in den Papierkorb und ist dann über den Client nicht mehr erreichbar.
	// ACHTUNG: Es ist implementationsabhängig, ob das Objekt wiederhergestellt werden kann! (Für Google Drive gilt: JA)
	Trash(fileId string) error

	// Save speichert die als Stream übergebenen Bytes als Datei ab.
	// Der fileName darf, je nach Implementation, mehrfach existieren.
	// D.h. eine gleichnamige Datei wird nicht zwangsweise überschrieben!
	// Mit maxRead kann die max. Anzahl an Bytes angegeben werden, die vom
	// Stream gelesen werden dürfen. 0 bedeutet dabei, lesen bis zu EOF.
	// Bei Erfolg wird die fileId der geschriebenen Datei zurück gegeben!
	Save(fileName string, file io.Reader, maxRead int64) (string, error)

	// InitFileList liest alle Dateien des "Speichers" ein und schreibt sie in eine interne Liste.
	// Diese Methode kann SEHR LANGSAM sein und muss MINDESTENS EINMAL aufgerufen werden!
	InitFileList() error

	// UpdateFileList aktualisiert die interne Liste, die durch InitFileList() erstellt wurde.
	// Diese Methode ist wesentlich performanter und lädt lediglich ein Delta (bei Google Drive).
	// Achtung: Es muss jedoch mindestens einmal InitFileList() aufgerufen worden sein!
	UpdateFileList() error

	// FileList gibt die interne Liste mit Dateien zurück.
	// Diese Methode ist offline und greift lediglich auf zwischengespeicherte Informationen zurück.
	// Für ein Update muss InitFileList() oder UpdateFileList() aufgerufen werden!
	// Der KEY dieser map ist die fileId, der VALUE ist ein FileObject mit allen wichtigen Daten.
	FileList() map[string]*FileObject // map key is the fileId
}

// FileObject kann eine Datei oder ein Ordner sein.
// Immer angegeben sind Id, Name und ModifiedTime.
// Size und Md5Checksum können bei einem Ordner entfallen.
type FileObject struct {
	// Mit der ID wird eine Datei oder ein Ordner eindeutig identifiziert. Der Name ist dafür nicht geeignet!
	// Beispiel von der Google Drive Implementierung: 1pl-ypWWGeF4sdgrlW-hH9PVz2gEvHWUJ
	Id string

	// Der Datei oder Ordnername des Objekts.
	// ACHTUNG: Es können verschiedene Objekte mit dem gleichen Namen existieren!
	Name string

	// Die letzte Änderung/Aktualisierung des Objekts.
	// Wurde eine Datei nie verändert, entspricht es dem Erstellungszeitpunkt.
	ModifiedTime time.Time

	// Die Dateigröße in Byte.
	// Bei Ordnern ist dieser Wert immer 0.
	Size int64

	// Der MD5-Hash als HEX-String.
	// Bei Ordnern ist dieser String leer.
	Md5Checksum string
}
