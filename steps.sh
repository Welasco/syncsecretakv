# Install Jekyll
sudo apt-get install ruby-full build-essential zlib1g-dev
gem install jekyll bundler

# Create a new Jekyll site
jekyll new syncsecretakv

# Build the site and make it available on a local server
cd jekyll-syncsecretakv
bundle exec jekyll serve --host=0.0.0.0

# Now you can browse to http://localhost:4000
# Stop the server by pressing Ctrl+C

# I customized the theme by changing the _config.yml file
# I also replaced the main page with index.markdown

# Install changes
bundle install

