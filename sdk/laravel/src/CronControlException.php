<?php

namespace CronControl\Laravel;

use RuntimeException;

class CronControlException extends RuntimeException
{
    public int $status;
    public string $errorCode;
    public string $hint;

    public function __construct(int $status, string $code, string $message, string $hint = '')
    {
        parent::__construct($message);
        $this->status = $status;
        $this->errorCode = $code;
        $this->hint = $hint;
    }
}
