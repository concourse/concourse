module Format exposing (..)


prependBeta : String -> String
prependBeta url =
    "/beta" ++ url
