use strict;
use warnings;
use utf8;
use Plack::Request;
use Router::Boom;
use Redis::Fast;
use Furl;
use Path::Tiny;
use JSON qw/decode_json encode_json/;
use Encode qw/encode_utf8 decode_utf8/;

my $redis = Redis::Fast->new;
my $furl = Furl->new;
my $router = Router::Boom->new;

$router->add("/favicon.ico", sub {
    return ["404", [], []];
});

$router->add("/", sub {
    my $file = path("index.html")->slurp;
    return ["200", ["Content-Type" => "text/html"], [$file]];
});

$router->add("/connect", sub {
    my $env = shift;
    my $req = Plack::Request->new($env);

    my $session = $req->parameters->{uuid};

    $redis->sadd("sessions", $session);

    return ["200", ["X-Kuiperbelt-Session" => $session], ["joined success"]];
});

$router->add("/recent", sub {
    my $env = shift;
    my $req = Plack::Request->new($env);

    my @messages = $redis->lrange("messages", 0, 20);
    my $data = encode_json([map { decode_utf8($_) } reverse @messages]);

    return ["200", ["Content-Type" => "application/json"], [$data]];
});

$router->add("/post", sub {
    my $env = shift;
    my $req = Plack::Request->new($env);

    my $message = $req->parameters->{message};
    $redis->rpush("messages", $message);

    for my $i (0..9) {
        my @sessions = $redis->smembers("sessions");
        my $resp = $furl->post(
            "http://localhost:12345/send",
            [map {; "X-Kuiperbelt-Session" => $_ } @sessions],
            $message,
        );
        last if $resp->status ne "400";

        my $data = decode_json($resp->content);
        my $errors = $data->{errors};
        last if !$errors || ref $errors ne "ARRAY";

        my @invalid_sessions = map { $_->{session} } @$errors;
        $redis->srem("sessions", @invalid_sessions);
    }

    return ["200", [], ["success post"]];
});


my $app = sub {
    my $env = shift;
    my ($dest) = $router->match($env->{PATH_INFO});
    $dest->($env);
};
