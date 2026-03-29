$services = @("catalog", "cart", "ordering", "inventory", "profiles", "reviews", "wishlists", "coupons")

Write-Host "Infrastucture kontrol ediliyor..." -ForegroundColor Green
# Infrastucture'i calistir (eger kapaliysa)
cd .. ; make up

# TODO: docker-compose infra ve services ayır. servisler duplicate olmayacak sekilde docker-compose services.yml gibi bir dosyada olsun. 
#bu sekilde sadece servisleri baslatmak istedigimizde docker-compose -f services.yml up -d yaparak sadece servisleri baslatabiliriz.
Write-Host "8 Adet Mikroservis Baslatiliyor..." -ForegroundColor Green
foreach ($svc in $services) {
    # Yeni bir powershell penceresinde bu spesifik mikroservisi terminali kapanmayacak sekilde ac
    Start-Process powershell -ArgumentList "-NoExit", "-Command", "`$host.UI.RawUI.WindowTitle='$svc Service'; cd ../src/services/$svc; go run ./cmd/server/"
}
Write-Host "Tum servislerin terminal pencereleri basariyla acildi! Siyah ekranlardaki 'listening' loglarini bekleyiniz." -ForegroundColor Yellow
