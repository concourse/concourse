module Main exposing (main)

import Browser
import Dict exposing (Dict)
import Dict.Extra as DE
import Html exposing (Html, div, text)
import Html.Attributes as HA
import Html.Events as HE
import Http
import Json.Decode as JD
import Json.Decode.Extra as JDE exposing (andMap)
import Query


type alias Doc =
    { tag : String
    , title : String
    , text : String
    , location : String
    }


type alias Model =
    { query : String
    , docs : BooklitIndex
    , result : Dict String Query.Result
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
            { model | result = DE.filterMap (match query) docs }


match : String -> String -> BooklitDocument -> Maybe Query.Result
match query tag doc =
    Query.matchWords query doc.title


type alias DocumentResult =
    { tag : String
    , result : Query.Result
    , doc : BooklitDocument
    }


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
        , Dict.toList model.result
            |> List.filterMap (\( tag, res ) -> Maybe.map (DocumentResult tag res) (Dict.get tag model.docs))
            |> List.sortWith suggestedOrder
            |> List.map (viewDocumentResult model)
            |> Html.ul [ HA.class "search-results" ]
        ]


suggestedOrder : DocumentResult -> DocumentResult -> Order
suggestedOrder a b =
    case compare a.doc.depth b.doc.depth of
        EQ ->
            case ( a.tag == a.doc.sectionTag, b.tag == b.doc.sectionTag ) of
                ( True, False ) ->
                    LT

                ( False, True ) ->
                    GT

                _ ->
                    compare (String.length a.doc.title) (String.length b.doc.title)

        x ->
            x


viewDocumentResult : Model -> DocumentResult -> Html Msg
viewDocumentResult model { tag, result, doc } =
    Html.li []
        [ Html.a [ HA.href doc.location ]
            [ Html.article []
                [ Html.div [ HA.class "result-header" ]
                    [ Html.h3 [] (emphasize result doc.title)
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


emphasize : Query.Result -> String -> List (Html Msg)
emphasize matches str =
    let
        ( hs, lastOffset ) =
            List.foldl
                (\( idx, len ) ( acc, off ) ->
                    ( acc
                        ++ [ Html.text (String.slice off idx str)
                           , Html.mark [] [ Html.text (String.slice idx (idx + len) str) ]
                           ]
                    , idx + len
                    )
                )
                ( [], 0 )
                matches
    in
    hs ++ [ Html.text (String.dropLeft lastOffset str) ]


decodeSearchIndex : JD.Decoder BooklitIndex
decodeSearchIndex =
    JD.dict decodeSearchDocument


decodeSearchDocument : JD.Decoder BooklitDocument
decodeSearchDocument =
    JD.succeed BooklitDocument
        |> andMap (JD.field "title" JD.string)
        |> andMap (JD.field "text" JD.string)
        |> andMap (JD.field "location" JD.string)
        |> andMap (JD.field "depth" JD.int)
        |> andMap (JD.field "section_tag" JD.string)
