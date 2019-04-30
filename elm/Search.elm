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
import Maybe.Extra as ME
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
    , result : Dict String DocumentResult
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


type DocumentMatch
    = TitleMatch Query.Result
    | TextMatch Query.Result


match : String -> String -> BooklitDocument -> Maybe DocumentResult
match query tag doc =
    Maybe.map TitleMatch (Query.matchWords query doc.title)
        |> ME.orElseLazy (\() -> Maybe.map TextMatch (Query.matchWords query doc.text))
        |> Maybe.map (DocumentResult tag doc)


type alias DocumentResult =
    { tag : String
    , doc : BooklitDocument
    , result : DocumentMatch
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
        , Dict.values model.result
            |> List.sortWith suggestedOrder
            |> List.map (viewDocumentResult model)
            |> Html.ul [ HA.class "search-results" ]
        ]


suggestedOrder : DocumentResult -> DocumentResult -> Order
suggestedOrder a b =
    case ( a.result, b.result ) of
        ( TitleMatch _, TextMatch _ ) ->
            LT

        ( TextMatch _, TitleMatch _ ) ->
            GT

        _ ->
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
                    [ Html.h3 []
                        [ case result of
                            TitleMatch m ->
                                emphasize m doc.title

                            TextMatch _ ->
                                Html.text doc.title
                        ]
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
                        [ case result of
                            TitleMatch _ ->
                                Html.text (String.left 130 doc.text)

                            TextMatch m ->
                                emphasize m (String.left 130 doc.text)
                        , if String.length doc.text > 130 then
                            Html.text "..."

                          else
                            Html.text ""
                        ]
                ]
            ]
        ]


emphasize : Query.Result -> String -> Html Msg
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
    Html.span [] (hs ++ [ Html.text (String.dropLeft lastOffset str) ])


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
