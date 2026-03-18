<?php

namespace CronControl\Laravel;

use Illuminate\Support\ServiceProvider;

class CronControlServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->mergeConfigFrom(__DIR__ . '/../config/croncontrol.php', 'croncontrol');

        $this->app->singleton(CronControlClient::class, function ($app) {
            return new CronControlClient(
                config('croncontrol.url'),
                config('croncontrol.api_key'),
                config('croncontrol.timeout', 30),
            );
        });
    }

    public function boot(): void
    {
        $this->publishes([
            __DIR__ . '/../config/croncontrol.php' => config_path('croncontrol.php'),
        ], 'croncontrol-config');
    }
}
