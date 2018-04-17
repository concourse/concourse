module Main exposing (..)


type alias BooklitIndex =
    Dict String BooklitDocument


type alias BooklitDocument =
    { title : String
    , text : String
    , location : String
    }


type alias Doc =
    { tag : String
    , title : String
    , text : String
    , location : String
    }
