# LocalCat

LocalCat adalah aplikasi chat lokal (LAN) berbasis Go dan Fyne. Aplikasi ini mencari perangkat LocalCat lain secara otomatis lewat UDP multicast, lalu mengirim pesan teks antar perangkat menggunakan koneksi TCP.

## Struktur

- `main.go` — antarmuka GUI Fyne.
- `internal/network` — discovery peer LAN, server TCP, dan format pesan.

## Menjalankan

```bash
go run .
```

## Build desktop

```bash
go build .
fyne package -os windows -icon Icon.png
```

## Build Android

```bash
fyne package -os android -appID dev.localcat.app -name LocalCat
```

Pastikan perangkat berada di jaringan LAN yang sama dan firewall mengizinkan UDP multicast serta koneksi TCP masuk.
