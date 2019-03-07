module Main exposing (main)

import Browser
import Dict exposing (Dict)
import Fuzzy
import Html exposing (Html, div, text)
import Html.Attributes as HA
import Html.Events as HE
import Http
import Json.Decode as JD
import Json.Decode.Extra as JDE exposing (andMap)


type alias Doc =
    { tag : String
    , title : String
    , text : String
    , location : String
    }


type alias Model =
    { query : String
    , docs : BooklitIndex
    , result : Dict String Fuzzy.Result
    }


type alias BooklitIndex =
    Dict String BooklitDocument


type alias BooklitDocument =
    { title : String
    , text : String
    , location : String
    , depth : Int
    , sectionTag : String
    }


type Msg
    = DocumentsFetched (Result Http.Error BooklitIndex)
    | SetQuery String


main : Program () Model Msg
main =
  Browser.element
        { init = always init
        , update = update
        , view = view
        , subscriptions = always Sub.none
        }

init : ( Model, Cmd Msg )
init =
    ( { docs = Dict.empty
      , query = ""
      , result = Dict.empty
      }
    , Cmd.batch
        [ Http.send DocumentsFetched <|
            Http.get "search_index.json" decodeSearchIndex
        ]
    )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        DocumentsFetched (Ok docs) ->
            ( performSearch { model | docs = docs }, Cmd.none )

        DocumentsFetched (Err err) ->
            (\a -> always a (Debug.log "failed to load index" err)) <|
                ( model, Cmd.none )

        SetQuery query ->
            ( performSearch { model | query = String.toLower query }, Cmd.none )


performSearch : Model -> Model
performSearch model =
    case ( model.query, model.docs ) of
        ( "", _ ) ->
            { model | result = Dict.empty }

        ( query, docs ) ->
            { model | result = Dict.map (match query) docs |> Dict.filter containsFuzzyChars }


match : String -> String -> BooklitDocument -> Fuzzy.Result
match query tag doc =
    let
        result =
            Fuzzy.match
              [ Fuzzy.insertPenalty 100
              , Fuzzy.movePenalty 100
              ]
              []
              query
              (String.toLower doc.title)
    in
    { result | score = result.score + (100 * doc.depth) }


view : Model -> Html Msg
view model =
    Html.div []
        [ Html.input
            [ HA.type_ "search"
            , HA.class "search-input"
            , HE.onInput SetQuery
            , HA.placeholder "Search the docs…"
            , HA.required True
            ]
            []
        , Html.ul [ HA.class "search-results" ] <|
            List.filterMap (viewResult model) <|
                List.sortBy (Tuple.second >> .score) (Dict.toList model.result)
        ]


containsFuzzyChars : String -> Fuzzy.Result -> Bool
containsFuzzyChars _ res =
    res.score < 1000


viewResult : Model -> ( String, Fuzzy.Result ) -> Maybe (Html Msg)
viewResult model ( tag, res ) =
    Dict.get tag model.docs
        |> Maybe.map (viewDocumentResult model ( tag, res ))


viewDocumentResult : Model -> ( String, Fuzzy.Result ) -> BooklitDocument -> Html Msg
viewDocumentResult model ( tag, res ) doc =
    Html.li []
        [ Html.a [ HA.href doc.location ]
            [ Html.article []
                [ Html.div [ HA.class "result-header" ]
                    [ Html.h3 [] (emphasize res.matches doc.title)
                    , if doc.sectionTag == tag then
                        Html.text ""

                      else
                        case Dict.get doc.sectionTag model.docs of
                            Nothing ->
                                Html.text ""

                            Just sectionDoc ->
                                Html.h4 [] [ Html.text sectionDoc.title ]
                    ]
                , if String.isEmpty doc.text then
                    Html.text ""

                  else
                    Html.p []
                        [ Html.text (String.left 130 doc.text)
                        , if String.length doc.text > 130 then
                            Html.text "..."

                          else
                            Html.text ""
                        ]
                ]
            ]
        ]


emphasize : List Fuzzy.Match -> String -> List (Html Msg)
emphasize matches str =
    let
        isKey index =
            List.foldl
                (\e sum ->
                    if not sum then
                        List.member (index - e.offset) e.keys

                    else
                        sum
                )
                False
                matches

        hl char ( acc, idx ) =
            let
                txt =
                    Html.text (String.fromChar char)

                ele =
                    if isKey idx then
                        Html.mark [] [ txt ]

                    else
                        txt
            in
            ( acc ++ [ ele ], idx + 1 )
    in
    Tuple.first (String.foldl hl ( [], 0 ) str)



decodeSearchIndex : JD.Decoder BooklitIndex
decodeSearchIndex =
    JD.dict decodeSearchDocument


decodeSearchDocument : JD.Decoder BooklitDocument
decodeSearchDocument =
    JD.succeed BooklitDocument
    |> andMap ( JD.field "title" JD.string)
    |> andMap ( JD.field "text" JD.string)
    |> andMap ( JD.field "location" JD.string)
    |> andMap ( JD.field "depth" JD.int)
    |> andMap ( JD.field "section_tag" JD.string)
