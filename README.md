# Intro 
This is a crawler for a website which has a limit: how many requests per second it can do.
It just prints found urls on the website to console.

# Running
```
docker build -t test-crawler . && docker run test-crawler https://some-website.com 2
```

here are 2 arguments at the end:
1)  https://some-website.com - a website to scan
2) 2 - requests amount per second 
