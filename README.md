# Файлы для итогового задания

В директории `tests` находятся тесты для проверки API, которое должно быть реализовано в веб-сервере.

Директория `web` содержит файлы фронтенда.

```
docker build -t go-todo .
```

```powershell
docker run -p 127.0.0.1:7540:7540 `
  -v "${PWD}:/app/data" `
  -e TODO_PASSWORD="the_hardest_password_have_ever_been" `
  -e TODO_SECRETKEY="the_hardest_secretkey_have_ever_been" `
  go-todo
```

```bash
docker run -p 127.0.0.1:7540:7540 \
  -v "${PWD}:/app/data" \
  -e TODO_PASSWORD="the_hardest_password_have_ever_been" \
  -e TODO_SECRETKEY="the_hardest_secretkey_have_ever_been" \
  go-todo
```