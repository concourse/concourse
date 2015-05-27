#lang scheme/base

(require racket/cmdline
         scribble/core
         scribble/html-properties
         scriblib/render-cond)

(provide version)

(define version-param (make-parameter "x.x.x"))
(define analytics-id (make-parameter ""))

(command-line
   #:once-each
   ["--version" v "Version of Concourse the docs are for" (version-param v)]
   ["--analytics-id" i "Google Analytics site ID" (analytics-id i)])

(define version (version-param))

(provide inject-analytics)

(define (inject-analytics)
  (cond-element
    [latex ""]
    [html (make-element
            (make-style #f
              (list
                (make-script-property
                  "text/javascript"
                  (analytics-script (analytics-id)))))
            '())]
    [text ""]))

(define (analytics-script site-id)
  (if (equal? "" site-id)
    (list)
    (list
      "(function(i,s,o,g,r,a,m){i['GoogleAnalyticsObject']=r;i[r]=i[r]||function(){\n"
      "(i[r].q=i[r].q||[]).push(arguments)},i[r].l=1*new Date();a=s.createElement(o),\n"
      "m=s.getElementsByTagName(o)[0];a.async=1;a.src=g;m.parentNode.insertBefore(a,m)\n"
      "})(window,document,'script','//www.google-analytics.com/analytics.js','ga');\n"
      "\n"
      "ga('create', '" site-id "', 'auto');\n"
      "ga('send', 'pageview');\n")))
