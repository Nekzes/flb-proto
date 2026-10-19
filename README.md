# flb-proto (Fast Light Binary Protocol) v2

Минималистичный бинарный протокол передачи данных поверх UDP для Go без сторонних зависимостей.

Вторая версия полностью переведена на датаграммы UDP со снижением оверхеда заголовка до 8 байт. Протокол включает встроенный модульный роутер сервера (Handlers), поддержку битовых флагов (OneWay, NeedACK, IsACK) и механизм гарантированной доставки с автоматической переотправкой по таймауту.

---

## Формат пакета в сети

Заголовок имеет фиксированный размер 8 байт (BigEndian). Размер полезной нагрузки определяется операционной системой из размера UDP-датаграммы:

+---------+--------+---------------+-------+-----------------+--------------------+
| Version | Prefix | Status_prefix | Flags |       Seq       |        Data        |
|  1 байт | 1 байт |    1 байт     | 1 байт| 4 байта (uint32)| N байт (до 1442 Б) |
+---------+--------+---------------+-------+-----------------+--------------------+

* Version — версия протокола (текущая: 2).
* Prefix — опкод команды (тип операции).
* Status_prefix — статус выполнения операции.
* Flags — битовая маска параметров доставки.
* Seq — уникальный порядковый номер пакета для сопоставления ACK.
* Data — полезная нагрузка.

---

## Константы

### Опкоды команд (Prefix)
```go
const (
    PrPing uint8 = 1 // Запрос проверки доступности
    PrPong uint8 = 2 // Ответ на проверку доступности
    PrEcho uint8 = 3 // Эхо-запрос / ответ с возвратом данных
)
```

### Статусные коды (Status_prefix)
```go
const (
    StPrError uint8 = 0 // Ошибка выполнения операции
    StPrDone  uint8 = 1 // Успешное выполнение операции
)
```

### Битовые флаги (Flags)
```go
const (
    FlagNone    uint8 = 0      // Без специальных флагов
    FlagOneWay  uint8 = 1 << 0 // 00000001: Fire-and-forget (сервер не отправляет ответ)
    FlagNeedACK uint8 = 1 << 1 // 00000010: Требуется подтверждение доставки (ACK)
    FlagIsACK   uint8 = 1 << 2 // 00000100: Служебный пакет подтверждения доставки
)
```

### Параметры протокола
```go
const (
    Version       uint8 = 2
    MaxBufferSize       = 2048
    MaxAttemp           = 10
    HeaderSize          = 8
    timeout             = 300 * time.Millisecond
)
```

---

## Структуры данных

### Package
Основная структура сетевого пакета.
```go
type Package struct {
    Version       uint8  // Версия протокола
    Prefix        uint8  // Опкод команды
    Status_prefix uint8  // Статус выполнения
    Flags         uint8  // Битовые флаги
    Seq           uint32 // Порядковый номер
    Data          []byte // Полезная нагрузка
}
```

### Client
Клиентская обёртка поверх net.Conn. Содержит атомарный счётчик пакетов Seq и внутренний буфер для чтения.

```go
type Client struct {
    Connect net.Conn
    Buf     []byte
    Seq     atomic.Uint32
}
```

### Server
Серверный роутер поверх *net.UDPConn. Обеспечивает чтение датаграмм, автоматическую обработку флагов и маршрутизацию по таблице Handlers.

```go
type Server struct {
    Connect  *net.UDPConn
    Buf      []byte
    Handlers map[uint8]HandlerFunc
}
```

### HandlerFunc
Сигнатура функции-обработчика для сервера. Если функция возвращает nil, ответ клиенту в сеть не отправляется (глушение).

```go
type HandlerFunc func(pkg *Package, addr net.Addr) *Package
```

---

## Описание функций и методов

### Конструктор пакета

#### NewPackage

```go
func NewPackage(data_bytes []byte, prefix_data uint8, stat_prefix uint8, flags uint8, seq uint32) *Package
```

Создаёт структуру *Package с предзаполненной версией Version = 2.

---

### Сериализация

#### (*Package) Pack

```go
func (pkg *Package) Pack() []byte
```

Сериализует структуру Package в непрерывный срез байтов: 8 байт заголовка плюс сырые данные.

#### UnPack

```go
func UnPack(d []byte) (*Package, error)
```

Разбирает сырой срез байтов датаграммы в структуру *Package. Возвращает ошибку, если длина среза меньше 8 байт.

---

### Методы Client

ВАЖНО: Методы Write и WriteACK принимают сырые байты ([]byte). Если вы работаете со структурой *Package, перед передачей в эти методы обязательно вызывайте метод pkg.Pack()!

#### NewClient

```go
func NewClient(conn net.Conn) *Client
```

Инициализирует клиента с переданным net.Conn и выделяет буфер размером 2048 байт.

#### (*Client) Write

```go
func (c *Client) Write(d []byte) error
```

Отправляет сырые байты пакета напрямую в UDP-сокет без ожидания ответа. Принимает результат выполнения pkg.Pack().

#### (*Client) WriteACK

```go
func (c *Client) WriteACK(d []byte) error
```

Гарантированная отправка с подтверждением доставки. Распаковывает переданные байты, автоматически инкрементирует Seq, выставляет флаг FlagNeedACK и отправляет в сеть. Ожидает входящий пакет с флагом FlagIsACK и совпадающим Seq в течение 300 мс. При отсутствии ответа производит до 10 повторных попыток отправки.

#### (*Client) Read

```go
func (c *Client) Read() (*Package, error)
```

Блокирует выполнение до получения датаграммы из сокета, считывает данные и автоматически возвращает распарсенный *Package.

#### (*Client) Close

```go
func (c *Client) Close() error
```

Закрывает сетевое соединение клиента.

---

### Методы Server

#### NewServer

```go
func NewServer(conn *net.UDPConn) *Server
```

Инициализирует сервер, создаёт буфер и внутреннюю таблицу роутера Handlers.

#### (*Server) Handle

```go
func (s *Server) Handle(prefix uint8, h_func HandlerFunc)
```

Регистрирует обработчик на указанный опкод команды (Prefix).

#### (*Server) Read

```go
func (s *Server) Read() ([]byte, net.Addr, error)
```

Считывает сырую датаграмму из сокета, создаёт независимую копию байтов (защита от race condition) и возвращает её вместе с адресом отправителя.

#### (*Server) WriteTo

```go
func (s *Server) WriteTo(d []byte, addr net.Addr) error
```

Отправляет срез байтов (результат pkg.Pack()) конкретному клиенту по его сетевому адресу net.Addr.

#### (*Server) ListenAndServe

```go
func (s *Server) ListenAndServe() error
```

Запускает бесконечный цикл обработки входящих UDP-пакетов:
1. Вычитывает датаграмму и парсит через UnPack.
2. Если во входящем пакете выставлен FlagNeedACK, моментально отправляет квитанцию с флагом FlagIsACK и тем же номером Seq.
3. Находит функцию в Handlers по опкоду Prefix.
4. Если обработчик вернул nil или во входящем пакете стоял флаг FlagOneWay, ответ в сеть не отправляется.
5. В остальных случаях автоматически упаковывает и отправляет возвращённый *Package клиенту.

#### (*Server) Close

```go
func (s *Server) Close() error
```

Останавливает сервер и закрывает слушающий UDP-сокет.

---

## Установка

```bash
go get github.com/Nekzes/flb-proto/flb
```

---

## Примеры использования

### Сервер с роутингом

```go
package main

import (
    "fmt"
    "net"

    "github.com/Nekzes/flb-proto/flb"
)

func main() {
    addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
    if err != nil {
        panic(err)
    }

    conn, err := net.ListenUDP("udp", addr)
    if err != nil {
        panic(err)
    }

    srv := flb.NewServer(conn)
    defer srv.Close()

    // 1. Регистрация PING (возврат ответа)
    srv.Handle(flb.PrPing, func(pkg *flb.Package, addr net.Addr) *flb.Package {
        fmt.Printf("[PING] от %s (Seq=%d)\n", addr.String(), pkg.Seq)
        return flb.NewPackage([]byte("PONG"), flb.PrPong, flb.StPrDone, flb.FlagNone, pkg.Seq)
    })

    // 2. Регистрация ECHO (возврат присланных данных)
    srv.Handle(flb.PrEcho, func(pkg *flb.Package, addr net.Addr) *flb.Package {
        fmt.Printf("[ECHO] от %s: %s\n", addr.String(), string(pkg.Data))
        return flb.NewPackage(pkg.Data, flb.PrEcho, flb.StPrDone, flb.FlagNone, pkg.Seq)
    })

    fmt.Println(">>> Сервер запущен на 127.0.0.1:8080")
    if err := srv.ListenAndServe(); err != nil {
        panic(err)
    }
}
```

### Клиент: отправка данных

```go
package main

import (
    "fmt"
    "net"

    "github.com/Nekzes/flb-proto/flb"
)

func main() {
    conn, err := net.Dial("udp", "127.0.0.1:8080")
    if err != nil {
        panic(err)
    }

    client := flb.NewClient(conn)
    defer client.Close()

    // 1. Обычная отправка пакета через Write (ОБЯЗАТЕЛЬНО вызываем .Pack())
    msg := flb.NewPackage([]byte("Hello Server"), flb.PrEcho, flb.StPrDone, flb.FlagNone, 0)
    if err := client.Write(msg.Pack()); err != nil {
        panic(err)
    }

    // Чтение ответа
    resp, err := client.Read()
    if err != nil {
        panic(err)
    }
    fmt.Printf("Ответ сервера: %s\n", string(resp.Data))

    // 2. Гарантированная отправка через WriteACK (также передаём msg.Pack())
    importantMsg := flb.NewPackage([]byte("Critical Data"), flb.PrEcho, flb.StPrDone, flb.FlagNone, 0)
    if err := client.WriteACK(importantMsg.Pack()); err != nil {
        fmt.Println("Ошибка доставки (ACK не получен):", err)
    } else {
        fmt.Println("Пакет успешно доставлен и подтверждён сервером")
    }

    // 3. Отправка без ожидания ответа (OneWay)
    oneWayMsg := flb.NewPackage([]byte("Telemetry data"), flb.PrEcho, flb.StPrDone, flb.FlagOneWay, 0)
    if err := client.Write(oneWayMsg.Pack()); err != nil {
        panic(err)
    }
}
```
