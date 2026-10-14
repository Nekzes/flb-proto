# flb-proto (Fast Light Binary Protocol)



Минималистичный бинарный протокол передачи данных поверх TCP (или любого io.Reader / io.Writer) для Go без сторонних зависимостей.



Решает проблему склеивания и фрагментации TCP-пакетов через фиксированный 10-байтовый заголовок с префиксом длины (Length-Prefixed Framing).



---



## Формат пакета в сети



Каждое сообщение состоит из 10 байт заголовка и полезной нагрузки переменной длины:




| Version | 1 байт | uint8 | Версия протокола (по умолчанию 1) |

| Prefix | 1 байт | uint8 | Тип команды / статус / опкод (0–255) |

| Len | 8 байт | uint64 (BigEndian) | Размер полезной нагрузки в байтах |

| Data | N байт | []byte | Сырые данные сообщения |



---



## Структура пакета



```go

type Package struct {
    Data    []byte // Полезные данные
    Prefix  uint8  // Опкод / тип команды
    Len     uint64 // Длина данных в байтах
    Version uint8  // Версия протокола
}

```



---



## Описание функций



### NewMessage

Создаёт новый экземпляр пакета с установленной версией и автоматически вычисленной длиной.

```go

func NewMessage(data_bytes []byte, prefix_data uint8) *Package{}
```


### (*Package) WriteData

Упаковывает заголовок (10 байт) и тело сообщения в единый срез байтов и отправляет за один системный вызов.

```go
func (p *Package) WriteData(wr io.Writer) error {}
```



### ReadData

Вычитывает из потока сначала 10 байт заголовка, извлекает длину и гарантированно дочитывает указанный объём данных тела через io.ReadFull.

```go
func ReadData(re io.Reader) ([]byte, error) {}
```



---



## Установка



```bash

go get github.com/Nekzes/flb-proto/flb

```



---



## Пример использования



### Отправка пакета

```go

package main

import (
    "net"
    "github.com/Nekzes/flb-proto/flb"
)


func main() {
    conn, err := net.Dial("tcp", "127.0.0.1:8080")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // Создаём пакет с префиксом команды 1
    pkg := flb.NewMessage([]byte("ping"), 1)

    // Отправляем в сокет
    if err := pkg.WriteData(conn); err != nil {
        panic(err)
    }
}

```



### Приём пакета

```go

package main

import (
    "fmt"
    "net"
    "github.com/Nekzes/flb-proto/flb"
)

func main() {
    listener, err := net.Listen("tcp", "127.0.0.1:8080")
    if err != nil {
        panic(err)
    }
    defer listener.Close()

    conn, err := listener.Accept()
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // Вычитываем данные из сокета
    data, err := flb.ReadData(conn)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Получено: %s\n", string(data))
}

```
